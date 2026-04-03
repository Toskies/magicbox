# Go sync 避坑指南

`sync` 包里的工具都很基础，但越基础越容易被“顺手一写”写出隐藏 bug。

这类 bug 的特点通常是：

- 平时不一定复现
- 一到并发高的时候出问题
- 表现成死锁、数据竞争、偶发 panic、Wait 卡死

这份文档重点讲 `sync` 包几个最常见工具的高频避坑点。

如果你想进一步理解它们为什么会这样工作，尤其是 `Mutex` / `RWMutex` 的快慢路径、状态位、饥饿模式与 semaphore 协作，建议配合阅读：

- [Go 锁与读写锁的底层实现解析](./go-mutex-rwmutex-source.md)

## 1. `Mutex` 和 `RWMutex` 不能复制后继续使用

错误示例：

```go
type Counter struct {
	mu sync.Mutex
	n  int
}

c1 := Counter{}
c2 := c1

c2.mu.Lock()
```

`sync.Mutex`、`sync.RWMutex`、`sync.WaitGroup`、`sync.Once` 这类同步原语都不应该在开始使用后再被复制。

复制后会导致内部状态分裂，结果非常不可预测。

避坑原则：

- 含锁结构体尽量用指针传递。
- 锁初始化后，不要复制整个对象。

## 2. `Unlock` 没配对，会直接 panic

错误示例：

```go
var mu sync.Mutex
mu.Unlock() // panic
```

或者：

```go
mu.Lock()
mu.Unlock()
mu.Unlock() // panic
```

锁不是引用计数。每次 `Lock` 只能对应一次 `Unlock`。

最稳妥的写法通常是：

```go
mu.Lock()
defer mu.Unlock()
```

前提是临界区不要太大、性能上能接受。

## 3. 锁的粒度过大，容易把并发写成串行

错误倾向：

```go
mu.Lock()
defer mu.Unlock()

callRPC()
writeDB()
doHeavyWork()
```

如果把耗时操作都包在锁里，会导致：

- 并发度急剧下降
- 阻塞链变长
- 更容易形成锁等待

避坑原则：

- 锁只保护共享状态访问
- 不要把无关的慢操作放进临界区

## 4. `RWMutex` 不是“总比 `Mutex` 更快”

很多人会机械地把 `Mutex` 升级成 `RWMutex`，以为这是性能优化。

其实未必。

`RWMutex` 更适合：

- 读远多于写
- 临界区相对明确
- 竞争模式稳定

如果写很多、锁范围小、逻辑简单，普通 `Mutex` 反而常常更合适。

不要在没有证据的情况下默认 `RWMutex` 更优。

## 5. 不要尝试“读锁升级成写锁”

错误示例：

```go
rw.RLock()
if needWrite {
	rw.Lock() // 容易死锁或逻辑错误
}
```

`RWMutex` 不支持锁升级。

你不能一边持有读锁，一边再去申请写锁并指望自动升级。

正确思路通常是：

- 先释放读锁
- 再重新申请写锁
- 或者重构逻辑，避免升级需求

## 6. `WaitGroup.Add` 的时机错了，会导致 panic 或逻辑错误

错误示例：

```go
go func() {
	wg.Add(1)
	defer wg.Done()
	work()
}()

wg.Wait()
```

这里 `Add(1)` 放进 goroutine 里太晚了。

可能发生的情况：

- 主 goroutine 先执行到 `Wait()`
- 此时计数还是 0
- `Wait()` 直接返回
- 后面的 goroutine 还没真正纳入等待范围

正确写法：

```go
wg.Add(1)
go func() {
	defer wg.Done()
	work()
}()
```

## 7. `WaitGroup.Done` 多调一次会 panic

错误示例：

```go
wg.Add(1)
wg.Done()
wg.Done() // panic
```

典型报错是：

```text
sync: negative WaitGroup counter
```

所以：

- `Add` 和 `Done` 数量必须严格匹配
- 有分支提前返回时，更要保证 `Done` 逻辑正确

常见安全写法：

```go
wg.Add(1)
go func() {
	defer wg.Done()
	work()
}()
```

## 8. `WaitGroup` 不能替代错误传播和取消机制

`WaitGroup` 只能做一件事：

- 等一组 goroutine 结束

它不能直接处理：

- 错误聚合
- 提前取消
- 超时退出

如果你需要的是：

- 某个 goroutine 出错后全体停止
- 汇总首个错误

通常更适合考虑：

- `context.Context`
- `errgroup`

## 9. `Once` 不是“失败可重试”的初始化工具

示例：

```go
var once sync.Once

once.Do(func() {
	panic("init failed")
})
```

`sync.Once` 的语义是“只执行一次”。

即使 `Do` 里面 panic，后续也不会自动重试。

所以如果你的初始化逻辑需要“失败后允许重试”，就不能直接拿 `Once` 硬套。

## 10. `Cond` 容易被误用，通常不是第一选择

`sync.Cond` 适合较底层的条件等待场景，但它比 channel 更容易写错。

常见坑包括：

- 忘记在循环里检查条件
- `Signal/Broadcast` 时机不对
- 共享条件状态没在锁保护下读写

正确模式通常是：

```go
mu.Lock()
for !ready {
	cond.Wait()
}
mu.Unlock()
```

注意：

- 不是 `if !ready { cond.Wait() }`
- 而是 `for`

因为被唤醒后，条件未必真的满足。

## 11. `sync.Map` 不是普通 `map` 的无脑替代

很多人看到并发问题，就想把：

```go
map + mutex
```

直接替换成：

```go
sync.Map
```

但 `sync.Map` 更适合特定场景，例如：

- 读多写少
- key 集合动态
- 多 goroutine 并发访问明显

如果逻辑需要：

- 复杂类型安全封装
- 稳定的泛型体验
- 强业务约束

很多时候还是 `map + mutex` 更清楚。

## 12. `defer Unlock` 很稳，但别把错误逻辑藏进去

推荐写法通常是：

```go
mu.Lock()
defer mu.Unlock()
```

但要注意：

- 如果函数很长，锁可能被持有得过久
- 如果后面还会调用可能阻塞的逻辑，也会扩大临界区

所以 `defer Unlock` 是好习惯，但不是“自动最佳”。

要先想清楚锁该保护哪一段共享状态。

## 13. 常见安全写法

### 标准的 `WaitGroup` 启动方式

```go
wg.Add(1)
go func() {
	defer wg.Done()
	work()
}()
```

### 锁保护最小临界区

```go
mu.Lock()
v := state[key]
mu.Unlock()
```

### `Cond` 用循环等待条件

```go
mu.Lock()
for !ready {
	cond.Wait()
}
mu.Unlock()
```

### 含锁对象尽量用指针传递

```go
type Store struct {
	mu sync.Mutex
}

func NewStore() *Store {
	return &Store{}
}
```

## 14. 实战记忆法

看到 `sync` 相关代码时，可以先问自己：

1. 这个含锁对象有没有被复制？
2. `Add` 是不是发生在启动 goroutine 之前？
3. `Done` 是否一定和 `Add` 数量匹配？
4. 锁是不是持有得太久了？
5. 这里真的需要 `RWMutex` 吗？
6. 我是不是想用 `WaitGroup` 做它根本不负责的事？

## 15. 一份简短的避坑清单

- 含 `Mutex/RWMutex/WaitGroup/Once` 的对象是否被复制了？
- `Unlock` 是否和 `Lock` 严格配对？
- `WaitGroup.Add` 是否放在 goroutine 启动之前？
- 是否错误尝试了读锁升级写锁？
- `WaitGroup` 是否被误当成取消/错误传播工具？
- `sync.Map` 是否真的适合当前场景？

如果这些问题都提前想清楚，Go `sync` 的大多数坑都能避开。
