# Go context 避坑指南

`context` 是 Go 里控制超时、取消、请求链路信息传播的核心工具。

它看起来只是一个参数，但实际项目里很多 goroutine 泄漏、请求不退出、超时不生效、日志链路断裂，最后都能追到 `context` 用错。

这份文档重点讲 `context` 的高频避坑点。

## 1. 不要传 `nil context`

错误示例：

```go
func Do(ctx context.Context) {
	_ = ctx.Done()
}

Do(nil) // 运行时出问题
```

`context.Context` 是接口，传 `nil` 不会在编译期报错，但后面只要代码调用了 `Deadline`、`Done`、`Err`、`Value`，就可能直接 panic。

避坑原则：

- 永远不要传 `nil`。
- 不确定传什么时，用 `context.TODO()`。
- 作为根 context，用 `context.Background()`。

## 2. 创建了可取消 context，就要记得 `cancel()`

示例：

```go
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()
```

很多人以为“反正超时后也会自动结束”，于是省略 `cancel()`。

这通常不是好习惯。`cancel` 的作用不只是“主动取消”，还包括尽早释放关联资源，尤其是：

- 定时器
- 子 context 链路
- 等待 `Done` 的 goroutine

避坑原则：

- `WithCancel`
- `WithTimeout`
- `WithDeadline`

只要你创建了它们，通常就应该在合适位置 `defer cancel()`。

## 3. 不要把 `context` 存到结构体里长期持有

错误示例：

```go
type Service struct {
	ctx context.Context
}
```

`context` 的设计初衷是：

- 请求级别
- 调用链级别
- 短生命周期

它不适合被结构体长期保存，更不适合作为对象状态的一部分。

这样做常见问题：

- 请求结束后，结构体里还留着过期 context
- 代码很难判断当前 ctx 属于哪次调用
- 多并发请求时容易串上下文

正确方式通常是：

```go
func (s *Service) Do(ctx context.Context) error {
	return nil
}
```

把 `context` 作为方法参数沿调用链显式传递。

## 4. `context` 应该是第一个参数

Go 社区有非常稳定的约定：

```go
func Do(ctx context.Context, id string) error
```

而不是：

```go
func Do(id string, ctx context.Context) error
```

这不只是风格问题，还影响：

- 一致性
- 可读性
- 与标准库和社区生态的配合

避坑原则：

- `ctx` 放第一个参数
- 命名通常就叫 `ctx`
- 不要把它藏进可选参数或配置对象里

## 5. 不要把 `context.Value` 当成万能参数包

错误倾向：

```go
ctx = context.WithValue(ctx, "user_id", 123)
ctx = context.WithValue(ctx, "timeout", 5)
ctx = context.WithValue(ctx, "debug", true)
ctx = context.WithValue(ctx, "config", cfg)
```

`context.Value` 适合放的是：

- 请求级元数据
- 跨 API 边界的少量上下文信息
- tracing / request id / auth claims 这类信息

它不适合放：

- 业务参数
- 可选配置
- 大对象
- 所有函数都顺手往里塞的数据

否则会带来：

- 类型不透明
- 调用方不知道依赖了哪些 key
- 调试困难
- key 冲突

## 6. `context.WithValue` 的 key 不要用裸字符串

错误示例：

```go
ctx = context.WithValue(ctx, "user_id", 123)
```

这样容易和别的包产生 key 冲突。

推荐写法：

```go
type contextKey string

const userIDKey contextKey = "user_id"

ctx = context.WithValue(ctx, userIDKey, 123)
```

更稳妥一点，可以用私有自定义类型，避免跨包冲突。

## 7. 开了 goroutine，就要考虑它是否监听 `ctx.Done()`

错误示例：

```go
go func() {
	for {
		work()
	}
}()
```

如果这个 goroutine 跟某个请求绑定，而它根本不监听 `ctx.Done()`，请求即使已经取消，这个 goroutine 也可能继续活着。

这就是常见的 goroutine 泄漏来源。

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

如果 `work()` 本身会阻塞，更应该让 `work` 内部也支持 context。

## 8. 只创建超时 context，不向下传递，等于没用

错误示例：

```go
ctx, cancel := context.WithTimeout(ctx, time.Second)
defer cancel()

callDB(context.Background())
```

这里虽然创建了带超时的 `ctx`，但真正的下游调用却没用它，超时控制就完全失效了。

避坑原则：

- 你创建了新的 `ctx`，后续链路就要继续传这个新的 `ctx`
- 不要中途随手换回 `context.Background()`

## 9. 不要在库代码里随手用 `context.Background()` 截断链路

错误倾向：

```go
func (s *Service) Query(ctx context.Context) error {
	return s.repo.Query(context.Background())
}
```

这会直接切断调用链，导致：

- 上游取消失效
- 上游超时失效
- tracing / request id 丢失

正确原则：

- 上游传进来的 `ctx`，就继续往下传
- 只有在真正需要创建新根 context 的边界场景，才用 `Background`

## 10. `defer cancel()` 写在循环里要小心

错误示例：

```go
for _, item := range items {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	process(ctx, item)
}
```

这样 `cancel` 会等函数整体返回时才执行，而不是每轮循环结束就执行。

结果可能是：

- 大量定时器积压
- 资源释放延后

更稳妥的写法：

```go
for _, item := range items {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	process(ctx, item)
	cancel()
}
```

或者把每轮逻辑封进一个小函数，让 `defer` 的作用域正确收敛。

## 11. `ctx.Err()` 只能说明取消原因，不能代替业务错误

示例：

```go
select {
case <-ctx.Done():
	return ctx.Err()
}
```

`ctx.Err()` 常见值只有两种：

- `context.Canceled`
- `context.DeadlineExceeded`

它适合表示“请求被取消/超时”，但不适合拿来表达业务错误。

不要把所有失败都粗暴映射成 `ctx.Err()`，否则真正的错误上下文会丢失。

## 12. 不要把 `context` 当配置对象

错误倾向：

```go
ctx = context.WithValue(ctx, retryKey, 3)
ctx = context.WithValue(ctx, batchSizeKey, 100)
ctx = context.WithValue(ctx, logLevelKey, "debug")
```

如果这些值本质上是函数配置、业务参数、模块选项，那么它们应该通过：

- 函数参数
- 配置结构体
- option pattern

来传递，而不是塞进 `context`。

`context` 负责的是取消、超时和少量跨边界元数据，不是“万能背包”。

## 13. 常见安全写法

### 创建后及时释放

```go
ctx, cancel := context.WithTimeout(parent, time.Second)
defer cancel()
```

### 下游继续传递同一个链路 context

```go
func (s *Service) Handle(ctx context.Context) error {
	return s.repo.Query(ctx)
}
```

### goroutine 监听退出信号

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

### 自定义 key 类型

```go
type contextKey string
```

## 14. 实战记忆法

看到 `context` 相关代码时，可以先问自己：

1. 这里有没有把 `nil` 当 context 传下去？
2. 创建了新 context 之后，是否继续往下传了？
3. `cancel()` 是否一定会调用？
4. goroutine 是否会在 `ctx.Done()` 后退出？
5. 这里放进 `Value` 的东西，真的是上下文元数据吗？
6. 有没有某一层偷偷用 `Background()` 把链路截断了？

## 15. 一份简短的避坑清单

- 是否误传了 `nil context`？
- 是否创建了 timeout/cancel context 却忘了 `cancel()`？
- 是否把 context 存进结构体长期持有？
- 是否错误使用了 `context.Value` 传业务参数？
- 是否有 goroutine 没监听 `ctx.Done()`？
- 是否中途用 `context.Background()` 截断了上游链路？

如果这些问题都想清楚，Go `context` 的大多数坑都能避开。
