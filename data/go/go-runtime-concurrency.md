# Go 并发模型与 Runtime 心智模型

很多人学 Go，会把“会开 goroutine”误以为“会写并发程序”。

实际上 Go 并发真正难的地方在于：

- 任务如何调度
- 数据如何同步
- 生命周期如何收敛
- 错误、取消、超时如何传递

如果这些概念没建立好，项目里很容易出现：

- goroutine 泄漏
- 死锁
- 数据竞争
- CPU 飙高但吞吐很差
- 明明加了并发却更慢

这篇文档的目标，不是只讲语法，而是帮你建立一套能指导真实项目的心智模型。

## 1. 先理解 Go 的并发哲学

Go 的一句经典口号是：

> 不要通过共享内存来通信；要通过通信来共享内存。

这句话的意思不是“不要用锁”，而是说：

- 优先思考任务边界和数据所有权
- 减少多个 goroutine 共同修改同一份状态
- 尽量让数据通过 channel 或清晰接口流动

Go 不是反对锁，而是反对“状态到处共享，但边界没人说得清”。

## 2. GMP 调度模型：goroutine 为什么能这么轻

Go runtime 的经典调度模型可以简化理解为：

- `G`: goroutine，待执行任务
- `M`: machine，真正执行代码的 OS 线程
- `P`: processor，运行 Go 代码所需的调度上下文

直观理解：

- goroutine 不是直接绑在线程上的
- runtime 会把大量 goroutine 调度到少量线程上执行
- `P` 的数量通常由 `GOMAXPROCS` 控制

这带来的好处是：

- goroutine 创建成本远低于线程
- 高并发网络服务更容易写
- 阻塞、唤醒、抢占由 runtime 帮你承担很大一部分复杂度

但要记住：

- goroutine 轻，不代表可以无节制乱开
- goroutine 多了以后，调度、内存、上下文切换、排查复杂度都会上升

## 3. goroutine 的正确认知

### goroutine 适合什么

适合：

- I/O 并发
- 多任务并发等待
- 独立 worker 处理
- pipeline / fan-out / fan-in 模式

不适合：

- 只是为了“看起来高级”而把顺序代码拆得很碎
- 每个请求里面无上限创建子 goroutine
- 没有退出条件的后台任务

### goroutine 不是免费资源

虽然 goroutine 栈会动态增长，但 goroutine 本身仍然会带来：

- 栈内存
- 调度成本
- 排查难度
- 泄漏风险

一个高质量 Go 项目，不是“goroutine 越多越厉害”，而是：

- 开得有边界
- 生命周期可控
- 能观察、能取消、能退出

相关专题见：

- [Go goroutine 避坑指南](./go-goroutine-pitfalls.md)

## 4. channel：它是同步原语，不只是队列

很多人把 channel 只理解为“消息队列”，这是不完整的。

channel 在 Go 里同时承担几种角色：

- 数据传递
- 同步信号
- 任务编排
- 关闭广播

### 无缓冲 channel

```go
ch := make(chan int)
```

发送和接收会彼此同步。

适合：

- 强同步
- 明确交接时机
- 保证某个动作已经被对方接收

### 有缓冲 channel

```go
ch := make(chan int, 16)
```

适合：

- 解耦生产与消费速度
- 短暂削峰
- worker pool 投递任务

但要记住：

- buffer 不是“性能万能药”
- 它只是把阻塞时机往后推
- 如果消费者跟不上，最终还是会堵

### `close(channel)` 的语义

关闭 channel 代表：

- 不会再有新的发送
- 接收方可以继续把缓冲区读完

很重要的一条原则：

- 一般由发送方关闭
- 更准确地说，由“最后一个发送者”关闭

错误关闭往往会导致：

- `send on closed channel`
- 多个生产者互相踩

相关专题见：

- [Go channel 避坑指南](./go-channel-pitfalls.md)

## 5. `select`：并发控制的核心操作台

`select` 能同时等待多个通信事件：

```go
select {
case msg := <-ch:
	_ = msg
case <-ctx.Done():
	return ctx.Err()
}
```

它最常见的用途：

- 监听超时或取消
- 在多个 channel 间复用
- 实现非阻塞尝试
- 实现定时任务与停止信号协同

### `default` 要慎用

带 `default` 的 `select` 如果没有命中其他分支，会立刻走 `default`。

这意味着：

- 它可以实现“非阻塞尝试”
- 也非常容易写出忙等循环，导致 CPU 飙高

所以看到 `for { select { default: ... } }` 时，要警惕是否在空转。

## 6. `sync` 家族：什么时候该用锁

Go 不是“只用 channel，不用锁”。

如果你管理的是共享内存状态，锁通常是更直接、也更高效的选择。

### `sync.Mutex`

最常用的互斥锁。

适合：

- 保护共享 map / slice / struct 状态
- 临界区较小、访问逻辑清晰

### `sync.RWMutex`

读多写少时可能有价值，但不要机械套用。

如果写入频繁、竞争激烈、临界区复杂，`RWMutex` 不一定比普通 `Mutex` 更好。

### `sync.WaitGroup`

用来等待一组 goroutine 结束。

常见用法：

```go
var wg sync.WaitGroup
for i := 0; i < 3; i++ {
	wg.Add(1)
	go func() {
		defer wg.Done()
	}()
}
wg.Wait()
```

经验规则：

- `Add` 尽量在启动 goroutine 之前完成
- 不要复制 `WaitGroup`
- `WaitGroup` 只负责等待，不负责传播错误和取消

### `sync.Once`

适合做懒初始化、单次加载。

### `sync.Cond`

适合复杂条件同步，但在业务代码里使用频率低于 channel 和锁。

相关专题见：

- [Go sync 避坑指南](./go-sync-pitfalls.md)

## 7. `sync/atomic`：轻量，但要求极高

`atomic` 适合做：

- 计数器
- 标志位
- 少量简单状态

不适合做：

- 复杂对象一致性维护
- 多字段联动事务式更新

如果你的共享状态已经复杂到需要解释很多句才能说清楚，通常应该考虑锁或更明确的所有权模型，而不是继续堆 `atomic`。

## 8. `context`：并发程序的生命周期总线

在 Go 服务里，`context.Context` 往往承担这些职责：

- 超时控制
- 主动取消
- 请求级元数据传递
- 跨 goroutine 生命周期联动

最典型的用法：

```go
ctx, cancel := context.WithTimeout(parent, time.Second)
defer cancel()
```

然后把这个 `ctx` 一路传下去。

### `context` 解决的是“何时停止”

它不负责业务参数传递，也不负责保存大对象。

`context` 最适合表达的问题是：

- 这个请求是否已经超时
- 调用方是否已经取消
- 这一整条调用链是否还应继续

相关专题见：

- [Go context 避坑指南](./go-context-pitfalls.md)

## 9. Go 内存模型：为什么“看起来没问题”的并发代码会出错

并发 bug 之所以难，是因为“你以为的执行顺序”不一定真存在。

Go 内存模型强调的是“同步先行关系”，也就是常说的 happens-before。

常见能建立同步关系的操作包括：

- channel 发送与接收
- mutex 的解锁与加锁
- 某些原子操作
- 明确的同步原语配合

如果两个 goroutine 并发访问同一份数据，其中至少一个在写，并且没有正确同步，就属于数据竞争。

数据竞争的危险在于：

- 有时测试跑不过
- 有时线上才出
- 有时看起来“偶尔错一下”
- 有时 `-race` 一跑就暴露

### `-race` 是排查并发问题的第一把刀

```bash
go test -race ./...
```

它不是万能的，但对于大量共享内存读写错误，都能提供非常高的发现价值。

## 10. 几个非常常见的并发模式

### Worker Pool

适合：

- 任务很多
- 并发数需要限制
- 每个任务处理逻辑相对独立

典型结构：

- 一个任务 channel
- 固定数量 worker goroutine
- 一个结果 channel 或外部存储

### Fan-out / Fan-in

适合：

- 一份任务拆给多个 worker 并行处理
- 最终再汇总结果

### Pipeline

适合：

- 多阶段处理
- 每一阶段职责清晰
- 阶段之间用 channel 连接

### `errgroup` 协同

在真实项目里，`golang.org/x/sync/errgroup` 非常实用。

它把几件事情结合在一起：

- 启动多个 goroutine
- 聚合错误
- 基于 `context` 协同取消

这往往比手写 `WaitGroup + error channel + cancel` 更稳。

## 11. 并发设计里的几个关键原则

### 原则一：先定义数据所有权，再谈并发

你应该先回答：

- 这份数据归谁修改
- 谁只读
- 谁负责关闭 channel
- 谁负责停止 worker

如果这些边界不清晰，代码即使“暂时能跑”，后面也很难维护。

### 原则二：优先控制 goroutine 生命周期

每开一个 goroutine，都要能回答：

- 它什么时候退出
- 谁负责通知它退出
- 如果下游阻塞，它会不会卡死
- 如果上游取消，它会不会继续跑

### 原则三：不要把 channel 当万能胶

channel 很好用，但不是任何同步问题都要上 channel。

一些场景用锁反而更简单：

- 保护缓存
- 保护计数器对象
- 保护共享状态机

### 原则四：先写正确，再调优

并发优化最容易犯的错是：

- 还没确认瓶颈，就盲目加 goroutine
- 还没证明锁慢，就疯狂改无锁结构

正确顺序应该是：

1. 先保证正确
2. 再做压测
3. 再做 profile
4. 最后定位热点后优化

## 12. 并发问题的典型症状与排查路径

### 症状：程序卡住不退出

优先排查：

- 是否有 goroutine 在等 channel
- 是否有锁没释放
- 是否有 `WaitGroup` 计数不平

### 症状：偶发数据错乱

优先排查：

- 是否存在共享变量无同步写入
- 是否误用循环变量闭包
- 是否多个 goroutine 改同一个 map

### 症状：goroutine 数量持续上涨

优先排查：

- 是否开了后台 goroutine 但不监听 `ctx.Done()`
- 是否下游阻塞导致发送方堆积
- 是否消费者退出后生产者还在继续发

### 症状：CPU 很高但吞吐不高

优先排查：

- 是否有忙等循环
- 是否有锁竞争
- 是否 goroutine 调度过度
- 是否无意义地创建大量短生命周期 goroutine

## 13. 和现有专题文档的搭配阅读顺序

如果你已经遇到具体问题，可以这样联动阅读：

- goroutine 不退出：先看本篇第 3、8、11 节，再看 [Go goroutine 避坑指南](./go-goroutine-pitfalls.md)
- channel 堵塞或 close 出错：先看本篇第 4、5 节，再看 [Go channel 避坑指南](./go-channel-pitfalls.md)
- 锁、共享状态问题：先看本篇第 6、9 节，再看 [Go sync 避坑指南](./go-sync-pitfalls.md)
- 请求超时或取消失效：先看本篇第 8 节，再看 [Go context 避坑指南](./go-context-pitfalls.md)

## 14. 一句话总结

Go 并发编程的本质，不是“会不会写 `go func()`”，而是：

- 知道 runtime 如何调度
- 知道状态如何同步
- 知道生命周期如何结束
- 知道出问题时如何验证、定位和收敛
