# Go channel 避坑指南

`channel` 是 Go 并发模型里的核心工具，但它并不是“只要会发会收就够了”。

很多并发 bug 不会直接报错，而是表现成：

- 程序卡住
- goroutine 泄漏
- 偶发 panic
- 数据没处理完却提前退出

这份文档重点不讲语法，而是集中讲 `channel` 的高频避坑点。

## 1. 向 `nil channel` 收发会永久阻塞

示例：

```go
var ch chan int

ch <- 1      // 永久阻塞
fmt.Println(<-ch) // 永久阻塞
```

这不是 panic，而是阻塞。

所以 `nil channel` 比较危险，因为它不像 `nil map` 写入那样立刻炸，而是更容易把程序静默卡死。

避坑原则：

- 发送前确认 channel 已初始化。
- 如果 channel 可能为 `nil`，逻辑上要明确它是“未初始化”还是“故意禁用”。

正确写法：

```go
ch := make(chan int)
```

## 2. 关闭已关闭的 channel 会 panic

错误示例：

```go
close(ch)
close(ch) // panic
```

`close` 不是幂等操作。

这在多 goroutine 共享一个 channel 时很容易发生，尤其是多个地方都“顺手”想关掉它。

避坑原则：

- 谁负责发送，通常就由谁负责关闭。
- 关闭动作要有明确所有权。
- 不要让多个 goroutine 随意竞争 `close`。

如果确实需要防重复关闭，通常用更高层的同步手段，比如 `sync.Once`。

## 3. 向已关闭的 channel 发送会 panic

错误示例：

```go
close(ch)
ch <- 1 // panic
```

这是 channel 最常见的 panic 场景之一。

很多人会问：

- 能不能先判断 channel 是否关闭，再决定发不发？

一般不靠谱。因为你即使判断了，下一瞬间也可能被别的 goroutine 关闭。

更稳妥的办法是从设计上保证：

- 只有发送方关闭 channel
- 关闭后不会再有发送动作

## 4. 从已关闭的 channel 接收，不会 panic

示例：

```go
v, ok := <-ch
```

如果 channel 已关闭并且缓冲区也读空了：

- `v` 是元素类型零值
- `ok` 是 `false`

例如：

```go
ch := make(chan int)
close(ch)

v, ok := <-ch
fmt.Println(v, ok) // 0 false
```

这点很重要，因为很多逻辑错误都来自“把零值误当成正常数据”。

只要“零值”和“channel 已关闭”有歧义，就一定要用双返回值接收。

## 5. `for range ch` 会一直读到 channel 被关闭

示例：

```go
for v := range ch {
	fmt.Println(v)
}
```

这个循环只有在 `ch` 被关闭后才会退出。

常见坑：

- 生产者已经不再发送了
- 但 channel 没有关闭
- 消费者 `range` 一直卡住

这类问题经常表现为：

- 主流程看起来都结束了
- 但程序就是不退出

避坑原则：

- 只要消费者用 `range ch`，发送方就要有明确的关闭时机。
- 如果没有自然关闭点，就不要用 `range`，改成 `select` + `done` 或其他协议。

## 6. 无缓冲 channel 需要发送和接收同时就绪

示例：

```go
ch := make(chan int)
ch <- 1 // 如果此时没人接收，就阻塞
```

无缓冲 channel 不是“普通队列”，它更像一次同步握手。

发送方和接收方必须配对成功，通信才能完成。

这意味着：

- 它适合做同步协作。
- 它不适合在没有消费者的情况下先塞一堆数据。

如果你真正需要暂存一定数量的数据，应该考虑缓冲 channel。

## 7. 有缓冲 channel 也不是“永远不会阻塞”

错误理解：

```go
ch := make(chan int, 3)
```

很多人会以为“有缓冲就不会阻塞”，这是错的。

实际规则是：

- 缓冲没满时，发送可以继续
- 缓冲满了，发送仍然会阻塞
- 缓冲为空时，接收仍然会阻塞

所以缓冲 channel 只是改变了阻塞发生的时机，不是取消阻塞。

## 8. `select` 加 `default` 容易写出忙等

错误示例：

```go
for {
	select {
	case v := <-ch:
		fmt.Println(v)
	default:
	}
}
```

这段代码在 `ch` 没数据时不会阻塞，而是疯狂空转，占用 CPU。

这就是典型的 busy loop。

避坑原则：

- `default` 不是“更高级的非阻塞”，它常常意味着主动轮询。
- 没有明确理由时，不要随手给 `select` 加 `default`。

如果你真的需要非阻塞检查，也要考虑加：

- `time.Sleep`
- 定时器
- 更合适的事件通知机制

## 9. `nil channel` 在 `select` 里会永久禁用该分支

示例：

```go
var ch chan int

select {
case v := <-ch:
	fmt.Println(v)
case <-time.After(time.Second):
	fmt.Println("timeout")
}
```

这里 `ch` 这个分支永远不会被选中，因为它对应的是 `nil channel`。

这是坑，也是技巧。

它常被用来动态启用/禁用某个 `select` 分支，但如果你不是故意这么设计，就很容易把逻辑写成“某个 case 永远不可能发生”。

## 10. channel 关闭不等于“对方已经处理完”

很多人把 `close(ch)` 理解成“任务结束通知”，但这只说明：

- 不会再有新数据发送进来

它不 automatically 表示：

- 消费者已经处理完全部数据
- 相关 goroutine 已经退出

例如：

```go
close(ch)
```

此时消费者可能还在继续读取缓冲区里剩下的数据。

如果你需要的是“所有 worker 都执行完成”，应该配合：

- `sync.WaitGroup`
- `errgroup`
- 独立的完成通知

不要把 `close` 和“完全结束”混为一谈。

## 11. 不消费 channel，容易造成 goroutine 泄漏

示例：

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

如果调用方拿到这个 channel 之后根本不读，或者只读一部分就丢掉，发送方 goroutine 很可能卡在发送操作上，永远退不出来。

这就是非常常见的 goroutine 泄漏来源。

避坑原则：

- 只要你启动了往 channel 发送数据的 goroutine，就要考虑没人消费时怎么退出。
- 长生命周期 pipeline 最好带 `context.Context` 或 `done` 通知。

## 12. `time.After` 放进大循环里，容易造成额外开销

示例：

```go
for {
	select {
	case <-ch:
	case <-time.After(time.Second):
	}
}
```

这类代码每一轮都会创建一个新的定时器。

在高频循环里，这通常不是好主意。

更稳妥的方式是复用 `time.Timer` 或 `time.Ticker`。

例如：

```go
ticker := time.NewTicker(time.Second)
defer ticker.Stop()
```

## 13. 关闭 channel 的职责要清晰

实践中最容易混乱的不是发送和接收，而是“到底谁来 close”。

一个实用原则：

> channel 应该由发送方关闭，而不是接收方关闭。

原因很简单：

- 发送方最清楚“不会再发了”这个事实。
- 接收方通常不知道后面是否还会来数据。

如果接收方贸然关闭 channel，很容易把仍在发送的 goroutine 直接送进 panic。

## 14. 常见安全写法

### 安全读取并判断是否关闭

```go
v, ok := <-ch
if !ok {
	return
}
```

### 生产者发完后关闭

```go
go func() {
	defer close(ch)
	for _, v := range data {
		ch <- v
	}
}()
```

### `range` 消费直到结束

```go
for v := range ch {
	process(v)
}
```

### 用 `context` 控制退出

```go
select {
case <-ctx.Done():
	return
case v := <-ch:
	_ = v
}
```

## 15. 实战记忆法

看到 `channel` 相关代码时，可以先问自己：

1. 这个 channel 会不会是 `nil`？
2. 谁负责关闭它？
3. 关闭后，是否还可能继续发送？
4. 消费者是不是一定能退出？
5. 这里会不会写成 busy loop？
6. 这里需要的是“数据传递”还是“完成通知”？

## 16. 一份简短的避坑清单

- 向 `nil channel` 收发会不会把流程卡死？
- 是否存在重复 `close` 或关闭后继续发送？
- `range ch` 是否一定能等到关闭？
- 是否错误地把 `close` 当成“所有工作已完成”？
- 是否可能因为没人消费而导致 goroutine 泄漏？
- `select + default` 是否变成了空转？

如果这些问题都想清楚，Go `channel` 的大多数坑都能避开。
