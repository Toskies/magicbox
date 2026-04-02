# Go goroutine 避坑指南

goroutine 很轻量，但“轻量”不等于“可以随便开”。

很多并发 bug 都不是 goroutine 本身难，而是生命周期、退出条件、错误传播、资源回收没设计清楚。最后表现成：

- 程序不退出
- goroutine 数量不断上涨
- 请求已经结束，后台还在跑
- 错误没有传回来
- 压力一大就把机器打满

这份文档重点讲 goroutine 的高频避坑点。

## 1. main 提前退出，子 goroutine 会直接跟着结束

示例：

```go
func main() {
	go func() {
		fmt.Println("worker")
	}()
}
```

这段代码不保证能打印出 `worker`。因为 `main` 返回后，整个进程就结束了，其他 goroutine 也会被直接终止。

避坑原则：

- 只要主流程依赖 goroutine 结果，就要有明确的等待机制。
- 常见方式是 `WaitGroup`、channel、`errgroup`。

## 2. 开了 goroutine，却没有退出条件，最容易泄漏

错误示例：

```go
go func() {
	for {
		work()
	}
}()
```

如果这里的 goroutine 绑定在某次请求、某个任务、某个 worker 生命周期上，但没有退出条件，它就很容易一直活着。

这类问题常见来源：

- 没监听 `ctx.Done()`
- 没有 stop 信号
- 阻塞在 channel 上没人唤醒

更稳妥的写法：

```go
go func() {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			work()
		}
	}
}()
```

前提仍然是 `work()` 自己不要无限阻塞，否则退出信号也接不到。

## 3. goroutine 阻塞在发送或接收上，没人管，就会卡死

错误示例：

```go
func producer() <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := 0; i < 10; i++ {
			ch <- i
		}
	}()
	return ch
}
```

如果调用方根本不消费这个 channel，或者读到一半就不读了，发送方 goroutine 很可能卡在 `ch <- i` 上再也退不出来。

这就是典型 goroutine 泄漏。

避坑原则：

- 发送方要考虑“没人接收怎么办”。
- 长生命周期 goroutine 最好支持 `context` 取消。
- 设计 pipeline 时，要明确中途退出协议。

## 4. 每次请求都随手起 goroutine，容易失控

错误倾向：

```go
for _, job := range jobs {
	go handle(job)
}
```

这在数据量小的时候看起来没问题，但一旦 `jobs` 很多，就会变成：

- goroutine 数量暴涨
- 调度开销上升
- 内存占用增加
- 下游资源被打爆

更稳妥的方式通常是：

- worker pool
- 带缓冲的任务队列
- 并发数限制

例如通过信号量限制并发：

```go
sem := make(chan struct{}, 8)

for _, job := range jobs {
	sem <- struct{}{}
	go func(job Job) {
		defer func() { <-sem }()
		handle(job)
	}(job)
}
```

## 5. 闭包捕获可变变量，逻辑很容易跑偏

这是 goroutine 里非常常见的逻辑错误。

示例：

```go
for i := 0; i < 3; i++ {
	go func() {
		fmt.Println(i)
	}()
}
```

这里 goroutine 捕获的是外层变量 `i`，而不是“当前这一轮的值副本”。实际输出往往不是你以为的 `0 1 2`。

更稳妥的写法：

```go
for i := 0; i < 3; i++ {
	i := i
	go func() {
		fmt.Println(i)
	}()
}
```

或者：

```go
for i := 0; i < 3; i++ {
	go func(i int) {
		fmt.Println(i)
	}(i)
}
```

注意：Go 1.22+ 已经修正了 `for range` 新声明变量的一类捕获问题，但“闭包捕获外层可变变量”这个风险本身并没有消失。

## 6. 子 goroutine 的错误不会自动回到调用方

错误示例：

```go
func run() error {
	go func() {
		if err := work(); err != nil {
			fmt.Println(err)
		}
	}()
	return nil
}
```

这里 `run()` 返回 `nil`，并不代表 goroutine 里的工作没失败。

goroutine 里的错误默认不会自动汇总到调用方，必须显式设计错误返回机制。

常见做法：

- 用 error channel 回传
- 用 `errgroup.Group`
- 把错误写入共享状态并同步等待

## 7. `recover` 只能在同一个 goroutine 里兜住 panic

错误理解：

```go
defer func() {
	if r := recover(); r != nil {
		fmt.Println("recovered")
	}
}()

go func() {
	panic("boom")
}()
```

外层 goroutine 的 `recover` 兜不住子 goroutine 的 panic。

如果 goroutine 内部可能 panic，而你又不想让整个进程被它打崩，就必须在那个 goroutine 自己内部处理：

```go
go func() {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()
	work()
}()
```

不过更本质的原则仍然是：

- 业务错误优先走 `error`
- 不要把 `panic` 当普通错误通道

## 8. goroutine 和 context 脱钩，请求结束后还在跑

错误示例：

```go
func Handle(ctx context.Context) {
	go func() {
		doSlowWork()
	}()
}
```

如果 `doSlowWork` 跟当前请求相关，而它根本不使用 `ctx`，那么请求超时或取消后，它仍然会继续执行。

这类问题经常带来：

- 重复写库
- 无意义外部请求
- 后台 goroutine 堆积

正确做法通常是把 `ctx` 一路传进去，并在阻塞点响应取消。

## 9. 不要让 goroutine 默默“后台化”而没有归属

错误倾向：

```go
go syncUser(userID)
```

这类 fire-and-forget 写法只有在非常明确的场景才成立。

你至少要想清楚：

- 它失败了怎么办？
- 它什么时候结束？
- 进程退出前要不要等它？
- 它是否会和后续请求并发冲突？

如果这些问题都没答案，那这个 goroutine 多半不是“轻量优化”，而是在制造不可控状态。

## 10. 不要把共享变量当成天然并发安全

错误示例：

```go
total := 0
for i := 0; i < 100; i++ {
	go func() {
		total++
	}()
}
```

这会有明显的数据竞争问题。

goroutine 轻量，不代表共享变量访问就自动安全。只要多个 goroutine 并发访问同一状态，就要明确使用：

- channel
- `sync.Mutex`
- 原子操作

以及配合 `go test -race` 检测。

## 11. 只启动不观测，出了问题很难定位

如果系统里 goroutine 很多，但你没有任何：

- 日志上下文
- 请求 ID
- 生命周期统计
- goroutine 数量观测

那么一旦出现泄漏或卡死，就很难知道是哪一类 goroutine 没退出。

最低限度建议：

- 长任务 goroutine 带日志标识
- 并发系统定期关注 goroutine 数量
- 有条件时接入 `pprof` 和 `trace`

## 12. 常见安全写法

### 用参数传值，避免闭包误捕获

```go
for i := 0; i < 3; i++ {
	go func(i int) {
		fmt.Println(i)
	}(i)
}
```

### 用 `WaitGroup` 等 goroutine 结束

```go
wg.Add(1)
go func() {
	defer wg.Done()
	work()
}()
wg.Wait()
```

### 用 `errgroup` 汇总错误和取消

```go
g, ctx := errgroup.WithContext(ctx)
g.Go(func() error { return work(ctx) })
if err := g.Wait(); err != nil {
	return err
}
```

### goroutine 内部监听取消

```go
go func() {
	select {
	case <-ctx.Done():
		return
	case msg := <-ch:
		_ = msg
	}
}()
```

## 13. 实战记忆法

看到 goroutine 相关代码时，可以先问自己：

1. 这个 goroutine 什么时候退出？
2. 如果调用方不再关心结果，它还会不会继续跑？
3. 错误怎么传回来？
4. 并发数会不会失控？
5. 它是否访问了共享状态？
6. 进程退出前需不需要等它？

## 14. 一份简短的避坑清单

- goroutine 是否有明确退出条件？
- 是否可能阻塞在 channel 收发上泄漏？
- 是否错误捕获了外层可变变量？
- 错误是否能回传，而不是默默丢失？
- 是否有并发数上限？
- 是否正确响应了 `context` 取消？

如果这些问题都想清楚，Go goroutine 的大多数坑都能避开。
