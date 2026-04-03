# Go 锁与读写锁的底层实现解析

这篇文档聚焦两个最常用的同步原语：

- `sync.Mutex`
- `sync.RWMutex`

和普通“怎么用锁”不同，这篇更关心它们在底层到底是怎么工作的。

## 0. 版本说明

以下分析基于当前本机 Go 源码：

- `go version go1.26.1 linux/amd64`

主要参考这些源码文件：

- `/usr/local/go/src/sync/mutex.go`
- `/usr/local/go/src/internal/sync/mutex.go`
- `/usr/local/go/src/sync/rwmutex.go`
- `/usr/local/go/src/sync/runtime.go`
- `/usr/local/go/src/runtime/sema.go`
- `/usr/local/go/src/runtime/rwmutex.go`

要注意一点：

- `sync.Mutex` 在当前版本里只是公开壳，真实实现已经放在 `internal/sync`。
- `RWMutex` 的公开实现仍然直接在 `sync/rwmutex.go`。
- 不同 Go 版本的实现细节可能会调整，所以你以后看源码时要先确认版本。

## 1. 为什么理解锁的底层实现很重要

如果你只会写：

```go
mu.Lock()
defer mu.Unlock()
```

那你只是“会用锁”。

但当项目里出现下面这些现象时，底层理解就开始有价值了：

- 并发上去后延迟突然变高
- 某段代码明明只是“加了个锁”，吞吐却掉得很厉害
- 为什么 `RWMutex` 有时候比 `Mutex` 还慢
- 为什么读锁不能升级成写锁
- 为什么 Go 的 `Mutex` 会提到饥饿模式

真正要回答这些问题，就得看源码里的：

- 状态位
- 自旋策略
- waiter 计数
- semaphore 休眠与唤醒
- 正常模式和饥饿模式切换

## 2. `sync.Mutex`：公开 API 很薄，核心实现很厚

当前版本的公开定义非常薄：

```go
type Mutex struct {
	_ noCopy
	mu isync.Mutex
}
```

也就是说：

- `sync.Mutex` 负责暴露稳定 API
- 真正的锁状态机在 `internal/sync.Mutex`

真实状态结构大致是：

```go
type Mutex struct {
	state int32
	sema  uint32
}
```

这里可以先建立两个直觉：

- `state` 负责记录锁当前的状态位和等待者数量
- `sema` 不是业务层的 semaphore，而是 runtime 用来挂起 / 唤醒 goroutine 的底层同步点

## 3. `Mutex.state` 里到底塞了什么

源码里的关键常量：

```go
const (
	mutexLocked = 1 << iota
	mutexWoken
	mutexStarving
	mutexWaiterShift = iota

	starvationThresholdNs = 1e6
)
```

可以把它理解成：

- `mutexLocked`：锁已经被持有
- `mutexWoken`：已经唤醒过一个 waiter，`Unlock` 不要再额外乱唤醒
- `mutexStarving`：当前进入饥饿模式
- 高位部分：等待者数量
- `starvationThresholdNs = 1e6`：等待超过约 1ms 时，可能切换到饥饿模式

也就是说，`Mutex` 不是只存一个“0/1 是否上锁”的布尔值，而是一个压缩状态机。

## 4. `Mutex.Lock` 的快路径

源码的快路径非常直接：

```go
if atomic.CompareAndSwapInt32(&m.state, 0, mutexLocked) {
	return
}
```

这说明在无竞争时：

- 只需要一次 CAS
- 不需要进 runtime 慢路径
- 不需要睡眠 / 唤醒

这也是为什么 Go 的 `Mutex` 在无竞争场景下非常轻。

### 快路径的含义

只要锁是空闲的：

1. 当前 goroutine 直接把 `state` 从 `0` 改成 `mutexLocked`
2. 成功后立刻返回

这类设计让大多数“低竞争短临界区”场景成本很低。

## 5. `Mutex.lockSlow`：真正的复杂度都在这里

一旦快路径失败，才会进入慢路径。

慢路径里有几个关键变量：

- `waitStartTime`
- `starving`
- `awoke`
- `iter`
- `old`

它们分别在表达：

- 我是不是已经等过一段时间
- 我是否认为自己已经进入饥饿态
- 我是不是已经被唤醒过一次
- 当前自旋了几轮
- 现在看到的旧状态是什么

### 第一步：能自旋就先自旋

源码里有一个很关键的判断：

```go
if old&(mutexLocked|mutexStarving) == mutexLocked && runtime_canSpin(iter) {
	...
	runtime_doSpin()
	...
}
```

可以直观理解为：

- 锁被持有，但还没进入饥饿模式
- 当前运行环境允许短暂自旋
- 那么先别急着睡眠，试着在 CPU 上等几轮

这样做的原因是：

- 如果持锁方很快就释放
- 自旋比挂起再唤醒更便宜

### `mutexWoken` 的作用

自旋时还会尝试设置 `mutexWoken`。

它在解决的问题是：

- 既然已经有一个 waiter 被唤醒去竞争锁了
- `Unlock` 就没必要再唤醒更多 goroutine

否则会造成无谓唤醒，增加调度与竞争成本。

## 6. 正常模式与饥饿模式

这是 `Mutex` 实现里最值得理解的一部分。

### 正常模式

正常模式下：

- waiter 按队列顺序等待
- 但被唤醒的 goroutine 不会直接拥有锁
- 它需要和“新来的 goroutine”继续竞争

这会带来一个现象：

- 新来的 goroutine 可能更占优势，因为它已经在 CPU 上运行

正常模式的优点是吞吐高，因为锁可以在活跃 goroutine 之间快速流转。

### 饥饿模式

源码注释明确写了：

- 如果 waiter 等待超过约 1ms
- mutex 可能切到 starvation mode

饥饿模式下：

- `Unlock` 不再只是“把锁放开”
- 而是把所有权更直接地移交给队头 waiter
- 新来的 goroutine 不再抢占这个机会

这样做的目的不是提高吞吐，而是控制尾延迟，避免某些 goroutine 一直抢不到锁。

### 为什么需要两种模式

如果只有正常模式：

- 吞吐高
- 但极端竞争下可能有人一直饿着

如果永远都走饥饿模式：

- 更公平
- 但性能会明显下降

所以 Go 的策略是：

- 默认偏高吞吐
- 等待太久再切到更公平的模式

## 7. waiter 是怎么睡下去的

在慢路径里，当前 goroutine 在决定入队后会调用：

```go
runtime_SemacquireMutex(&m.sema, queueLifo, 2)
```

它表达的不是“用户态 semaphore 编程”，而是：

- 把当前 goroutine 挂到 runtime 管理的等待点上
- 等后续 `Unlock` 来唤醒

这里的几个关键信号：

- `queueLifo := waitStartTime != 0`
- 说明如果我已经等过一次，再入队时可能更偏向队头策略
- 这是为了避免“老 waiter 一直输给新 goroutine”

也就是说，Go 的 `Mutex` 并不是一个简单粗暴的睡眠锁，而是带有公平性调节逻辑的自适应锁。

## 8. `Mutex.Unlock` 的快路径和慢路径

`Unlock` 的第一步很简单：

```go
new := atomic.AddInt32(&m.state, -mutexLocked)
```

也就是先把 locked 位去掉。

### 如果 `new == 0`

说明：

- 没有 waiter
- 没有额外状态

那就直接返回。

### 如果 `new != 0`

才进入 `unlockSlow`。

#### 正常模式下

`unlockSlow` 会检查：

- 有没有 waiter
- 是否已经有人被唤醒
- 是否已经被别的 goroutine 重新抢到锁
- 当前是否在饥饿模式

如果条件合适，它会：

1. 设置 `mutexWoken`
2. waiter 数减 1
3. 调用 `runtime_Semrelease(&m.sema, false, 2)` 唤醒一个 goroutine

#### 饥饿模式下

它会调用：

```go
runtime_Semrelease(&m.sema, true, 2)
```

这里 `handoff=true` 的语义非常关键：

- 当前解锁者不是简单广播“锁空了”
- 而是更明确地把执行机会交给等待者

源码注释也提到：

- 这样做后，解锁方会让出自己的时间片
- 让被交接到锁的 waiter 尽快运行

## 9. 从源码理解 `Mutex` 的几个行为特征

### `Mutex` 不是可重入锁

如果同一个 goroutine：

```go
mu.Lock()
mu.Lock()
```

第二次会直接把自己挂住。

原因不是“Go 讨厌可重入”，而是它的设计压根没有 goroutine owner 这一层记录。

### `Mutex` 也不是“绑定 goroutine 身份”的锁

公开文档明确说过：

- 一个 goroutine `Lock`
- 另一个 goroutine `Unlock`

在运行时语义上是允许的。

因为底层并不记录“这个锁属于谁”，它只维护状态机和等待队列。

这也是 Go 锁和很多带 owner 概念的运行库实现不太一样的地方。

### `Mutex` 不能复制

因为复制会直接把：

- `state`
- `sema`

这一整套内部状态复制走。

结果就是：

- 原对象和新对象都像“半真半假”的锁
- 状态机会被破坏

## 10. `RWMutex` 的结构：不是一把锁，而是一组协调机制

`RWMutex` 的结构比 `Mutex` 更值得细看：

```go
type RWMutex struct {
	w           Mutex
	writerSem   uint32
	readerSem   uint32
	readerCount atomic.Int32
	readerWait  atomic.Int32
}
```

可以这样理解：

- `w`：先把写者之间串行化，避免多个 writer 互相乱抢
- `writerSem`：writer 等待读者退出时睡眠的点
- `readerSem`：reader 等 writer 结束时睡眠的点
- `readerCount`：当前读者计数，也是“是否有 writer 挂起”的信号位
- `readerWait`：writer 正在等待多少个“还没走完的读者”

### `rwmutexMaxReaders`

源码里有：

```go
const rwmutexMaxReaders = 1 << 30
```

它的作用不是“真的允许十亿级读者”，而是作为一个很大的偏移量。

writer 到来时会把 `readerCount` 减去这个大数，以此向后续 reader 宣告：

- 现在有 writer pending
- 新 reader 不要再直接进了

## 11. `RLock` 为什么这么快

`RLock` 的快路径非常漂亮：

```go
if rw.readerCount.Add(1) < 0 {
	runtime_SemacquireRWMutexR(&rw.readerSem, false, 0)
}
```

直观理解：

1. 先把读者数量加一
2. 如果结果仍然是非负，说明没有 writer 宣告“我要写”
3. 这个 reader 直接成功

所以读锁快的关键，不是魔法，而是：

- 正常读场景只需要一个原子加

### 为什么小于 0 就要阻塞

因为 writer 会在 `Lock` 时做：

```go
r := rw.readerCount.Add(-rwmutexMaxReaders) + rwmutexMaxReaders
```

这一步相当于给 `readerCount` 打上了“writer pending”的标记。

于是之后新来的 reader 再 `Add(1)`，结果就会小于 0，于是它们要去 `readerSem` 上睡。

这也是 `RWMutex` 防 writer 饥饿的关键策略之一：

- 只要 writer 宣告自己来了
- 后来的 reader 就不能没完没了插队

## 12. `RUnlock` 做了什么

`RUnlock` 会先做：

```go
if r := rw.readerCount.Add(-1); r < 0 {
	rw.rUnlockSlow(r)
}
```

这说明：

- 普通场景下，读者只是把计数减一就结束
- 只有 writer pending 时，才进入慢路径

慢路径里的关键逻辑是：

```go
if rw.readerWait.Add(-1) == 0 {
	runtime_Semrelease(&rw.writerSem, false, 1)
}
```

含义是：

- 当前有 writer 在等所有 reader 退出
- 每个 reader 离开时都把 `readerWait` 减一
- 最后一个 reader 负责唤醒那个 writer

这就是典型的“最后离开的关灯”模式。

## 13. `Lock`：writer 是怎么拿到写锁的

`RWMutex.Lock` 的流程可以拆成 3 步。

### 第一步：先和其他 writer 竞争

```go
rw.w.Lock()
```

这一步确保：

- 同一时刻只有一个 writer 在进行后续流程

### 第二步：宣告“有 writer 来了”

```go
r := rw.readerCount.Add(-rwmutexMaxReaders) + rwmutexMaxReaders
```

这里 `r` 表示宣告前已有多少活跃 reader。

完成这步后：

- 旧 reader 可以继续退场
- 新 reader 会发现 `readerCount < 0`，于是阻塞

### 第三步：等旧 reader 清空

```go
if r != 0 && rw.readerWait.Add(r) != 0 {
	runtime_SemacquireRWMutex(&rw.writerSem, false, 0)
}
```

意思是：

- 如果当前没有 reader，writer 直接进
- 如果当前还有 `r` 个 reader，就把它们登记到 `readerWait`
- 最后一个 reader 走时负责唤醒 writer

所以 `RWMutex` 的写锁不是“把所有人一脚踢开”，而是：

1. 阻止新 reader 继续入场
2. 等老 reader 自然退出
3. 然后 writer 再独占

## 14. `Unlock`：writer 释放时为什么要循环唤醒 reader

writer 解锁时的关键代码：

```go
r := rw.readerCount.Add(rwmutexMaxReaders)
...
for i := 0; i < int(r); i++ {
	runtime_Semrelease(&rw.readerSem, false, 0)
}
rw.w.Unlock()
```

直观理解：

- 先把“writer pending”这个大偏移量加回去
- `r` 代表有多少 reader 被挡在门外
- 这些 reader 需要被逐个唤醒
- 最后再释放 `w`，允许下一个 writer 竞争

这也是为什么 `RWMutex` 在某些模式下不一定便宜：

- 如果读写切换频繁
- 被阻塞的 reader 很多
- writer 解锁时要做一批唤醒工作

这部分调度和唤醒成本未必比普通 `Mutex` 更划算。

## 15. 为什么 `RWMutex` 不支持锁升级和降级

源码和文档都明确表态：

- 读锁不能升级成写锁
- 写锁也不能自动降级成读锁

### 为什么不能升级

假设你持有一个读锁，还想原地升级为写锁。

问题在于：

- 你自己也在 reader 集合里
- writer 必须等待所有 reader 退出
- 于是你一边等写锁，一边又阻止自己所需的“所有 reader 退出”

这非常容易形成死锁或复杂歧义。

### 为什么不能递归读锁

文档里也特别强调：

- 当 writer pending 时，新的 `RLock` 会阻塞

所以如果同一个 goroutine 已经拿着读锁，又在某条路径里再次 `RLock`，而恰好这时有 writer 在等，它也可能把自己卡住。

这就是为什么 Go 官方明确说 `RWMutex` 不适合递归读锁。

## 16. `RWMutex` 与公平性：更准确地说是“抑制 writer 饥饿”

很多人会说：

- `RWMutex` 是读优先
- 或者是写优先

更准确的说法是：

- 它允许已有 reader 并发前进
- 但一旦 writer 宣告到来，就会阻止新的 reader 继续进入

这个策略的核心目标是：

- 避免 writer 永远被源源不断的新 reader 饿死

所以它不是绝对的写优先，而是更像：

- “已有读者先退场，新的读者先别进，给 writer 一个收口机会”

## 17. `RWMutex` 为什么不总比 `Mutex` 快

这是实际项目里最常见的误解之一。

虽然 `RWMutex` 允许并发读，但它也引入了更多机制成本：

- 多个计数器和 semaphore
- writer 到来时的读者阻塞协调
- 解锁时的批量 reader 唤醒
- 更复杂的状态转换

所以如果你的场景是：

- 写不少
- 临界区短
- 共享状态简单

普通 `Mutex` 经常更合适。

只有在这些条件同时成立时，`RWMutex` 才更容易赢：

- 读远远多于写
- 读临界区有一定长度
- 读操作确实值得并发

## 18. `sync` 包与 runtime 的关系

在 `sync/runtime.go` 里你能看到 `sync` 对 runtime 的一些桥接声明：

- `runtime_SemacquireRWMutexR`
- `runtime_SemacquireRWMutex`
- `runtime_Semrelease`

这说明：

- `sync` 包本身不自己实现 goroutine 的睡眠 / 唤醒调度
- 它把这件事委托给 runtime 的 semaphore 机制

你可以把分工粗略理解成：

- `sync`：定义锁语义和状态机
- runtime：负责调度、休眠、唤醒、性能细节

这也是 Go 并发原语设计里一个很重要的特点：

- 用户态 API 简单
- 复杂度被藏进 runtime 和内部包里

## 19. 和 `runtime/rwmutex.go` 的关系

如果你继续往下看，还会发现 runtime 里有一个自己的 `rwmutex` 实现。

它和公开版 `sync.RWMutex` 很像，但用途不同：

- runtime 版本用于调度器和运行时内部
- 它阻塞的是底层 M，而不是普通业务视角里的 goroutine 协作语义

所以你会看到两份“长得像”的实现并存：

- 一份给用户空间 `sync`
- 一份给 runtime 自己内部使用

## 20. 从源码反推日常开发建议

### 建议一：临界区尽量小

因为一旦有竞争：

- `Mutex` 可能开始自旋、入队、唤醒
- `RWMutex` 可能开始做 reader / writer 协调

锁住 RPC、I/O、长计算都会放大这些成本。

### 建议二：别默认 `RWMutex` 更高级

源码层面看，它更复杂，不是更“高级”。

### 建议三：高竞争问题不要靠猜

你可以结合：

- `pprof` 的 mutex profile
- block profile
- `go test -race`

用数据看看到底是：

- 锁粒度太大
- writer 被饿
- goroutine 太多
- 还是共享状态设计本身有问题

### 建议四：源码理解是为了写更简单的代码

理解底层不是为了在业务代码里玩花活，而是为了：

- 知道什么时候该用 `Mutex`
- 什么时候该用 `RWMutex`
- 什么时候该重构为“减少共享状态”

## 21. 搭配阅读建议

如果你准备把这块吃透，推荐顺序：

1. [Go 并发模型与 Runtime 心智模型](./go-runtime-concurrency.md)
2. [Go sync 避坑指南](./go-sync-pitfalls.md)
3. 本文
4. [Go 性能分析与 Profiling 方法](./go-performance-profiling.md)

这样你会把：

- 抽象心智模型
- 高频错误
- 源码实现
- 线上性能观察

这四层串起来。

## 22. 一句话总结

Go 的 `Mutex` 和 `RWMutex` 都不是“简单开关”。

它们背后分别站着：

- 原子状态机
- 自旋与休眠策略
- semaphore 唤醒机制
- 对吞吐、公平性、尾延迟的权衡

理解这层源码，不是为了手写锁，而是为了更准确地判断：

- 为什么现在会卡
- 为什么这个锁型不合适
- 为什么某段并发代码看起来正确，实际却容易在高竞争下出问题
