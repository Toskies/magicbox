# Go interface 避坑指南

`interface` 是 Go 里非常强大的抽象工具，但它也经常制造最难排查的 bug。

这些 bug 的特点通常是：

- 编译能过
- 运行时才出问题
- 代码表面看起来“类型对了”
- 实际语义却和你以为的不一样

这份文档重点讲 Go `interface` 的高频避坑点。

## 1. `nil interface` 和“装着 nil 指针的 interface”不是一回事

这是 Go `interface` 最经典的坑。

示例：

```go
type MyError struct{}

func (e *MyError) Error() string { return "err" }

func f() error {
	var e *MyError = nil
	return e
}
```

很多人会以为 `f()` 返回的是 `nil`，其实不一定。

原因是 interface 值内部通常包含两部分：

- 动态类型
- 动态值

当它的动态类型是 `*MyError`，动态值是 `nil` 时，这个 interface 本身仍然可能不等于 `nil`。

结果就是：

```go
if err != nil {
	// 明明觉得没错，结果进来了
}
```

避坑原则：

- 返回 `error` 时，无错误就直接 `return nil`
- 不要返回“带类型的 nil 指针”充当空接口值

## 2. 指针接收者会影响接口实现

示例：

```go
type Runner interface {
	Run()
}

type Job struct{}

func (j *Job) Run() {}
```

这时候：

- `*Job` 实现了 `Runner`
- `Job` 不一定实现 `Runner`

例如：

```go
var r Runner
r = Job{} // 编译错误
```

这不是 interface 本身有问题，而是方法集规则决定的。

避坑原则：

- 如果方法是指针接收者，就优先传 `*T`。
- 设计接口时，要清楚值类型和指针类型谁实现了它。

## 3. 类型断言单值形式可能 panic

错误示例：

```go
var x any = 123
s := x.(string) // panic
```

如果动态类型不是你断言的那个类型，单值断言会直接 panic。

更安全的写法：

```go
s, ok := x.(string)
if !ok {
	return
}
fmt.Println(s)
```

只要输入不是你完全可控的，优先使用 `comma-ok` 形式。

## 4. type switch 也要小心 `nil`

示例：

```go
switch v := x.(type) {
case nil:
	fmt.Println("nil")
case string:
	fmt.Println(v)
}
```

这里要注意：

- 只有 interface 本身为 `nil`，才会走 `case nil`
- 如果 interface 里装的是“带类型的 nil 指针”，通常不会走 `case nil`

这和上一节其实是同一个坑的另一种表现。

## 5. interface 比较不是总能安全执行

很多人以为 interface 都可以直接 `==` 比较，这是错的。

示例：

```go
var a any = []int{1, 2}
var b any = []int{1, 2}

fmt.Println(a == b) // panic
```

为什么：

- interface 比较最终要比较其动态值
- 但 `slice`、`map`、`func` 本身就是不可比较类型

所以如果 interface 里装的是不可比较值，直接比较就会 panic。

避坑原则：

- 不确定动态类型是否可比较时，不要直接 `==`
- 比较复杂对象时，用专门的比较逻辑，例如 `reflect.DeepEqual`

## 6. `any` 不是“可以不做设计”

`any` 只是 `interface{}` 的别名。

它的意义是：

- 这里可以放任意类型

它不意味着：

- 这里可以不考虑类型约束
- 这里的代码自然就更灵活

过度使用 `any` 常见后果：

- 到处都是类型断言
- 编译期检查减少
- 运行时错误变多
- 调用方看不懂到底该传什么

避坑原则：

- 能用具体类型就用具体类型
- 能用小接口就用小接口
- 只有在真正需要泛化时，再考虑 `any`

## 7. interface 装的是值副本，不一定是原对象本身

示例：

```go
type User struct {
	Name string
}

u := User{Name: "Tom"}
var x any = u

u.Name = "Jerry"
fmt.Println(x.(User).Name) // Tom
```

这里放进 interface 的是 `u` 当时的值副本。

如果你想让 interface 持有的是同一个可变对象，应该放指针：

```go
var x any = &u
```

这又会引入共享状态问题，所以要明确自己的语义：

- 需要值语义，放值
- 需要共享修改，放指针

## 8. interface 变量本身有零值，但方法调用不一定安全

示例：

```go
type Runner interface {
	Run()
}

var r Runner
r.Run() // panic
```

这里 `r` 的零值是 `nil interface`，直接调方法会 panic。

所以 interface 字段、interface 返回值、interface 依赖项，在使用前要确认它真的已经被赋值。

## 9. 不要把“大而全接口”当成好抽象

错误倾向：

```go
type Service interface {
	Create()
	Update()
	Delete()
	List()
	Export()
	Import()
}
```

这类接口常见问题：

- 实现方负担过重
- mock 很笨重
- 很多调用方只需要其中 1-2 个方法

Go 更推崇小接口。

例如：

```go
type Reader interface {
	Read(p []byte) (int, error)
}
```

避坑原则：

- 接口由使用方抽象，不是由实现方炫技扩展。
- 能拆小就拆小。

## 10. `error` 也是 interface，要按 interface 规则理解

很多人把 `error` 当作语言内建特殊类型，但它其实也是接口：

```go
type error interface {
	Error() string
}
```

因此 `error` 会继承 interface 的各种典型坑：

- `nil error` 和“装着 nil 指针的 error”不同
- 类型断言可能失败
- 指针接收者方法会影响是否实现 `error`

只要你看到 `error`，就要记得它本质上仍然是 interface。

## 11. type switch 并不能替代清晰的建模

示例：

```go
func handle(x any) {
	switch v := x.(type) {
	case int:
		_ = v
	case string:
		_ = v
	case []byte:
		_ = v
	case map[string]any:
		_ = v
	}
}
```

如果一个函数越来越依赖超长的 type switch，通常说明：

- 输入模型太散
- 设计缺少稳定抽象
- interface 用来兜底了，而不是用来表达能力边界

type switch 本身没问题，但如果它成了主逻辑中心，通常值得重新审视建模。

## 12. interface 适合表达能力，不适合表达数据结构

这是一个常见设计误区。

好接口通常表达的是“能做什么”：

```go
type Writer interface {
	Write(p []byte) (int, error)
}
```

而不是“长什么样”。

如果你只是想传递数据，通常更适合：

- 结构体
- 具体类型
- 泛型参数

而不是为了“灵活”直接上 `interface{}` / `any`。

## 13. 常见安全写法

### 返回空错误时直接 `nil`

```go
if ok {
	return nil
}
```

### 类型断言使用 `comma-ok`

```go
v, ok := x.(MyType)
if !ok {
	return
}
```

### 先判断接口是否为空

```go
if r == nil {
	return
}
```

### 小接口优先

```go
type Doer interface {
	Do()
}
```

### 需要共享状态时明确放指针

```go
var x any = &cfg
```

## 14. 实战记忆法

看到 interface 相关代码时，可以先问自己：

1. 这个 interface 真的是 `nil`，还是里面装了带类型的 `nil`？
2. 这里的类型断言会不会 panic？
3. 这个类型到底是值实现接口，还是指针实现接口？
4. 这里放进 interface 的是值副本还是共享对象？
5. 这个抽象真的需要 interface，还是具体类型更清楚？
6. 我是不是在用超大接口或超长 type switch 掩盖建模问题？

## 15. 一份简短的避坑清单

- 返回 `error` 时，是否误返回了带类型的 `nil`？
- 类型断言是否使用了安全形式？
- 指针接收者是否影响了接口实现？
- interface 比较时，动态值是否可能不可比较？
- 这里该用小接口、具体类型，还是 `any`？
- interface 里装的是副本还是共享对象？

如果这些问题都提前想清楚，Go `interface` 的大多数坑都能避开。
