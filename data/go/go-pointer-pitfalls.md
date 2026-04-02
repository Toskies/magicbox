# Go 指针避坑指南

Go 的指针比 C 简单很多，没有指针运算，但这并不代表它不容易踩坑。

真正让人出错的，通常不是 `*` 和 `&` 语法，而是下面这些问题：

- `nil` 指针解引用
- 循环变量取地址
- `range` 变量取地址
- 值接收者和指针接收者混用
- 函数里改了“副本”，以为改到了原值
- 指针共享导致状态被意外修改

这份文档重点讲 Go 指针在日常开发中的高频避坑点。

## 1. `nil` 指针解引用会直接 panic

最基础，也是最危险的坑。

错误示例：

```go
type User struct {
	Name string
}

var u *User
fmt.Println(u.Name) // panic
```

或者：

```go
var p *int
fmt.Println(*p) // panic
```

避坑原则：

- 只要一个指针可能为 `nil`，使用前就要明确判断。
- 如果某个字段允许为空，代码里要把“为空时怎么处理”写清楚。

推荐写法：

```go
if u == nil {
	return
}

fmt.Println(u.Name)
```

## 2. 返回局部变量地址不是问题，Go 会处理

很多刚从 C/C++ 过来的人会担心这个：

```go
func newInt() *int {
	x := 10
	return &x
}
```

在 Go 里这是合法的，不是悬空指针。

原因是 Go 编译器会做逃逸分析。如果局部变量在函数外还需要继续存活，它会被分配到堆上。

真正需要担心的不是“局部变量地址能不能返回”，而是：

- 你是否真的需要返回指针
- 这个对象是否会被多人共享和修改

## 3. `for range` 里的循环变量取地址，仍然要谨慎

错误示例：

```go
nums := []int{10, 20, 30}
ptrs := make([]*int, 0, len(nums))

for _, v := range nums {
	ptrs = append(ptrs, &v)
}

for _, p := range ptrs {
	fmt.Println(*p)
}
```

这段代码经常不会得到你想要的结果，因为 `v` 是循环变量副本，不是底层数组中那个元素本身。

在 Go 1.22 及以后，`for range` 中新声明的循环变量会按迭代分别创建，所以它不再是老版本里“所有指针都指向同一个 `v`”的那个坑。

但问题仍然存在：

- `&v` 取到的是循环变量副本的地址，不是切片元素本身的地址。
- 如果你后面想通过这个指针修改原切片元素，改不到原数据。

正确写法有两种。

### 写法一：取原切片元素地址

```go
for i := range nums {
	ptrs = append(ptrs, &nums[i])
}
```

### 写法二：每轮创建新变量

```go
for _, v := range nums {
	v := v
	ptrs = append(ptrs, &v)
}
```

但在需要指向原数据时，第一种更直接。

## 4. 不只是切片，`range map` 的 value 取地址也一样有坑

示例：

```go
m := map[string]int{"a": 1, "b": 2}
ptrs := make([]*int, 0)

for _, v := range m {
	ptrs = append(ptrs, &v)
}
```

这里的 `v` 同样只是循环变量副本，不是 `map` 内部元素本身。

而且 `map` 元素本来就不可取地址，所以更不能把这类代码当成“拿到了 map value 的指针”。

避坑原则：

- `range` 出来的 `v` 默认先当作副本看。
- 看到 `&v` 时，先警觉一次。

## 5. 值接收者和指针接收者混用，很容易让修改失效

示例：

```go
type Counter struct {
	N int
}

func (c Counter) Inc() {
	c.N++
}

func main() {
	c := Counter{}
	c.Inc()
	fmt.Println(c.N) // 0
}
```

这里 `Inc` 是值接收者，调用时改的是副本，不是原对象。

如果方法需要修改对象状态，通常应该用指针接收者：

```go
func (c *Counter) Inc() {
	c.N++
}
```

记忆方式：

- 只读方法，值接收者通常可以。
- 要修改状态，优先考虑指针接收者。
- 同一个类型的方法集不要随意混搭，除非你非常清楚这样做的语义。

## 6. 函数参数传的是指针，不等于“所有修改都能传回去”

很多人会混淆两件事：

- 修改指针指向的对象
- 修改指针变量本身

示例：

```go
func reset(p *int) {
	x := 0
	p = &x
}

func main() {
	v := 10
	p := &v
	reset(p)
	fmt.Println(*p) // 10
}
```

这里 `reset` 里改的是形参 `p` 这个局部副本，不是调用方手里的那个指针变量。

如果你想改“指针指向的内容”，应该这样：

```go
func reset(p *int) {
	*p = 0
}
```

如果你真的想改“调用方持有的指针本身”，那你需要二级指针，或者直接返回新指针。

## 7. 指针字段共享后，改一处可能到处都变

示例：

```go
type Config struct {
	Timeout *int
}

t := 30
a := Config{Timeout: &t}
b := a

*b.Timeout = 60

fmt.Println(*a.Timeout) // 60
```

这里 `a` 和 `b` 虽然是两个结构体值，但它们内部的指针字段指向同一块内存。

这类问题经常出现在：

- 配置对象复制
- DTO 转换
- 测试中复用样例对象
- 缓存和默认值合并

如果你需要真正独立的副本，就不能只做结构体浅拷贝。

## 8. 指针不是越多越好

很多 Go 初学者会把所有结构体都设计成 `*T` 到处传。

这通常会带来几个问题：

- `nil` 判断变多
- 共享可变状态变多
- 代码更难推理
- 测试更容易互相污染

适合用指针的常见场景：

- 对象较大，频繁拷贝成本高
- 方法需要修改接收者状态
- 需要明确表达“可空”
- 需要和某些接口或框架约定对齐

不一定非用指针的场景：

- 小结构体，只读传递
- 值语义更自然的数据对象
- 希望减少共享状态、降低副作用

原则不是“能用指针就用指针”，而是“确实需要共享或修改时再用指针”。

## 9. `new(T)` 不是构造函数的替代品

示例：

```go
u := new(User)
```

这只是分配一个零值 `User` 并返回指针，本身没有任何业务初始化逻辑。

如果你的对象需要满足某些约束，比如：

- 默认字段值
- 内部 map/slice 初始化
- 校验逻辑

那就更适合显式写构造函数：

```go
func NewUser(name string) *User {
	return &User{Name: name}
}
```

尤其是结构体里有 `map`、`slice`、`chan` 这类字段时，`new(T)` 经常只能给你一个“外壳不为 nil，内部字段还是零值”的对象。

## 10. 指针接收者方法，也可以被值调用，但别误解语义

示例：

```go
type Counter struct {
	N int
}

func (c *Counter) Inc() {
	c.N++
}

func main() {
	var c Counter
	c.Inc()
}
```

这段代码是合法的，因为编译器会在可取地址的情况下帮你做隐式取址。

但不要因此误以为：

- 值和指针在所有场景下都完全等价。

比如：

- 接口实现时，方法集规则会影响赋值是否成立。
- 不可取地址的临时值就没法这样调用。

典型坑：

```go
type Runner interface {
	Run()
}

type Job struct{}

func (j *Job) Run() {}

func main() {
	var r Runner
	r = Job{} // 编译错误
	_ = r
}
```

因为 `Job{}` 这个值类型的方法集里没有 `Run`，只有 `*Job` 才实现了接口。

## 11. 结构体嵌套切片、map、指针时，拷贝往往只是浅拷贝

示例：

```go
type User struct {
	Tags []string
}

u1 := User{Tags: []string{"go", "db"}}
u2 := u1

u2.Tags[0] = "java"
fmt.Println(u1.Tags) // [java db]
```

虽然这里没显式出现指针，但切片本身就是引用到底层数组的描述符，本质上和共享内部状态的问题是一样的。

同类风险也适用于：

- `map` 字段
- `*T` 字段
- 嵌套对象里的引用类型字段

如果对象里含有引用语义成员，就要先问自己：

> 这次复制，是不是只复制了外壳？

## 12. `nil` 接口和“内部装着 nil 指针的接口”不是一回事

这是 Go 里一个非常绕的坑。

示例：

```go
type MyError struct{}

func (e *MyError) Error() string { return "err" }

func f() error {
	var e *MyError = nil
	return e
}
```

很多人以为 `f()` 返回的是 `nil`，其实不一定。

原因是接口值由两部分组成：

- 动态类型
- 动态值

当你返回的是“类型为 `*MyError`、值为 `nil`”时，这个接口本身可能并不等于 `nil`。

典型后果：

```go
if err != nil {
	// 明明以为没错，结果进来了
}
```

避坑原则：

- 返回 `error` 时，如果没有错误，就直接 `return nil`
- 不要返回“带类型的 nil 指针”充当空接口值

## 13. 常见安全写法

### 判断指针是否为空

```go
if p == nil {
	return
}
```

### 修改对象状态时使用指针接收者

```go
func (u *User) Rename(name string) {
	u.Name = name
}
```

### 获取切片元素地址时用下标

```go
for i := range items {
	ptrs = append(ptrs, &items[i])
}
```

### 需要新对象时显式返回

```go
func NewConfig() *Config {
	return &Config{}
}
```

### 明确返回修改后的值或指针

```go
func ResetUser(u *User) {
	if u == nil {
		return
	}
	u.Name = ""
}
```

## 14. 实战记忆法

看到指针相关代码时，可以快速问自己：

1. 这里会不会是 `nil`？
2. 我改的是对象本身，还是只改了一个副本？
3. 这里取地址的是原数据，还是 `range` 变量？
4. 这份拷贝是不是只做了浅拷贝？
5. 这里真的需要指针吗，还是值语义更合适？
6. 返回给接口的，是不是一个“带类型的 nil”？

## 15. 一份简短的避坑清单

- 指针使用前是否做了 `nil` 判断？
- `range` 中是否错误地对循环变量取了地址？
- 需要修改状态的方法，是否应该使用指针接收者？
- 当前修改的是原对象，还是一个局部副本？
- 结构体复制后，内部指针/切片/map 是否仍然共享？
- 返回接口时，是否误返回了带类型的 `nil` 指针？

如果这些问题都提前想清楚，Go 指针的大多数坑都能绕开。
