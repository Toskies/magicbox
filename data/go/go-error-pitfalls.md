# Go error 避坑指南

Go 的错误处理很直接，但“直接”不代表“不会踩坑”。

真正常见的问题不是不会返回 `error`，而是：

- 错误被吞了
- 错误链断了
- 比较方式错了
- 上下文丢了
- 明明返回了 `error`，判断却失真

这份文档重点讲 Go `error` 的高频避坑点。

## 1. `nil error` 和“带类型的 nil error”不是一回事

这是最经典的坑之一。

示例：

```go
type MyError struct{}

func (e *MyError) Error() string { return "err" }

func f() error {
	var e *MyError = nil
	return e
}
```

很多人会以为 `f()` 返回的是 `nil`，其实返回的 interface 可能不等于 `nil`。

结果：

```go
if err != nil {
	// 意外进入
}
```

避坑原则：

- 没有错误时，直接 `return nil`
- 不要返回“带类型的 nil 指针”作为 `error`

## 2. 只打印错误，不处理错误，不叫处理

错误倾向：

```go
if err != nil {
	fmt.Println(err)
}
```

如果打印完继续跑，而这个错误本来就该阻断流程，那只是把错误“展示出来”，并没有真正处理。

处理错误通常意味着至少要做一件事：

- 返回它
- 包装后返回
- 重试
- 降级
- 记录后终止当前分支

不要把“打印了一下”误认为“处理过了”。

## 3. 包装错误时忘记 `%w`，错误链就断了

错误示例：

```go
return fmt.Errorf("query user failed: %v", err)
```

这样虽然把原始错误文本拼进去了，但错误链断了，后面没法再用 `errors.Is` / `errors.As` 识别底层错误。

正确写法：

```go
return fmt.Errorf("query user failed: %w", err)
```

记住：

- 想保留错误链，用 `%w`
- 只想打印文本，不做错误传播，才用 `%v`

## 4. 包装过的错误，不要再用 `==` 判断

错误示例：

```go
if err == sql.ErrNoRows {
	return nil
}
```

如果错误已经被包装过：

```go
err = fmt.Errorf("query failed: %w", sql.ErrNoRows)
```

这时 `==` 比较通常就失效了。

正确做法：

```go
if errors.Is(err, sql.ErrNoRows) {
	return nil
}
```

## 5. 想拿到具体错误类型，要用 `errors.As`

示例：

```go
var pathErr *os.PathError
if errors.As(err, &pathErr) {
	fmt.Println(pathErr.Path)
}
```

如果你关心的是：

- 某种错误类型
- 类型里的附加字段

就应该用 `errors.As`，而不是手写类型断言去赌错误没被包装。

## 6. 不要丢失错误上下文

错误示例：

```go
if err != nil {
	return err
}
```

这在某些底层函数里没问题，但在中上层业务逻辑里，直接裸返回常常会让定位变困难。

更好的写法通常是：

```go
if err != nil {
	return fmt.Errorf("create order failed: %w", err)
}
```

这样日志和调用链里会多出关键上下文。

原则不是“逢错必包”，而是：

- 在跨层返回时，尽量补上当前语义上下文

## 7. 不要过度包装，变成错误套娃

虽然错误要有上下文，但也不能每一层都机械地包一遍。

例如：

```go
return fmt.Errorf("repo failed: %w", err)
return fmt.Errorf("service failed: %w", err)
return fmt.Errorf("handler failed: %w", err)
```

如果这些信息全都很空泛，最后错误消息只会越来越长，却没有新增有效信息。

更好的原则是：

- 只在新增关键信息时包装
- 错误上下文要具体，不要空泛

例如：

```go
return fmt.Errorf("load user %d failed: %w", userID, err)
```

## 8. `defer` 里处理错误，容易覆盖原始错误

错误示例：

```go
func f() (err error) {
	defer func() {
		err = cleanup()
	}()

	err = doWork()
	return
}
```

如果 `doWork()` 已经失败，而 `cleanup()` 又返回另一个错误，这里原始错误就被覆盖了。

更稳妥的方式通常是：

- 只在原错误为空时再赋值
- 或者显式合并错误

例如：

```go
defer func() {
	if cerr := cleanup(); err == nil {
		err = cerr
	}
}()
```

Go 1.20+ 也可以考虑 `errors.Join`。

## 9. 忽略错误返回值，通常是在埋雷

错误倾向：

```go
data, _ := io.ReadAll(r)
_ = json.Unmarshal(data, &v)
```

短期看起来省事，长期往往最难排查。

一旦出问题，你连“失败发生在哪一步”都不清楚。

只有在你非常确定“这个错误可以安全忽略”时，才应该显式忽略，并且最好加注释说明原因。

## 10. `panic` 不是普通错误处理手段

`panic` 适合：

- 明确不可恢复的程序员错误
- 启动阶段致命配置错误
- 非预期不变量被破坏

它不适合代替日常业务错误返回。

错误倾向：

```go
if err != nil {
	panic(err)
}
```

如果这是普通业务失败，那通常应该返回 `error`，而不是把整个 goroutine 或进程打崩。

## 11. 自定义错误类型时，要想清楚判断方式

你可以用几种方式表达错误：

- 哨兵错误
- 自定义类型错误
- 动态包装错误

例如哨兵错误：

```go
var ErrNotFound = errors.New("not found")
```

例如类型错误：

```go
type ValidationError struct {
	Field string
}
```

选择时要想清楚：

- 调用方以后是要 `errors.Is` 判断？
- 还是要 `errors.As` 取结构化字段？

不要随手定义，后面全靠字符串匹配。

## 12. 不要用错误字符串做稳定逻辑判断

错误示例：

```go
if strings.Contains(err.Error(), "not found") {
	return nil
}
```

这非常脆弱，因为：

- 错误文本可能改
- 包装后格式可能变
- 不同语言或不同实现可能不一致

稳定判断应该优先用：

- `errors.Is`
- `errors.As`
- 明确的错误类型或哨兵错误

## 13. 常见安全写法

### 包装并保留错误链

```go
if err != nil {
	return fmt.Errorf("query order %d failed: %w", orderID, err)
}
```

### 判断包装链中的哨兵错误

```go
if errors.Is(err, sql.ErrNoRows) {
	return nil
}
```

### 提取具体错误类型

```go
var pathErr *os.PathError
if errors.As(err, &pathErr) {
	_ = pathErr.Path
}
```

### 合并多个错误

```go
err = errors.Join(err, cleanupErr)
```

## 14. 实战记忆法

看到 `error` 相关代码时，可以先问自己：

1. 这里返回的是不是一个真正的 `nil error`？
2. 这个错误是否需要补充上下文？
3. 我是不是用错了 `%v`，导致错误链断了？
4. 这里应该用 `errors.Is`、`errors.As` 还是直接返回？
5. `defer` 里的错误处理会不会覆盖原始错误？
6. 我是不是在拿错误字符串做逻辑判断？

## 15. 一份简短的避坑清单

- 是否误返回了带类型的 `nil error`？
- 是否忽略了本不该忽略的错误？
- 是否应该用 `%w` 保留错误链？
- 是否错误地用 `==` 判断了包装错误？
- 是否丢失了关键业务上下文？
- 是否错误依赖了 `err.Error()` 文本做判断？

如果这些问题都提前想清楚，Go `error` 的大多数坑都能避开。
