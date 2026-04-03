# Go 语言基础与常用法

Go 的门槛看起来不高，因为语法不复杂；但要真正把 Go 写“顺”，核心不是记语法，而是切换思维方式。

Go 倾向于：

- 简单、显式，而不是魔法多
- 组合优先，而不是继承优先
- 错误返回值优先，而不是异常流优先
- 小接口、清晰边界，而不是层层抽象

如果你带着 Java、Python、Node.js 的习惯来写 Go，通常不是写不出来，而是容易写出“不像 Go”的代码。

## 1. 先建立 Go 的几个核心心智模型

### Go 不是“语法简化版的 Java”

Go 的核心关注点是工程效率和可维护性，不是语言特性越多越好。

典型表现：

- 没有类继承体系
- 没有异常作为主错误流
- 接口实现是隐式的
- 倾向于组合、显式依赖、清晰数据流

所以学 Go 时最容易犯的错，不是某个关键字不会用，而是总想把它写成别的语言。

### Go 重视“零值可用”

Go 里很多类型都强调零值即可工作，比如：

- `sync.Mutex`
- `bytes.Buffer`
- `map` 以外的大多数结构体字段组合

这会影响你的设计方式：

- 尽量让结构体初始化简单
- 少依赖复杂构造器
- 尽量让“默认状态”也是合法状态

### Go 倾向于数据流显式传递

例如：

- 错误显式返回
- `context.Context` 作为第一个参数显式传递
- 依赖通过结构体字段或函数参数注入

这让代码看起来啰嗦一点，但换来的好处是：

- 调试更直接
- 调用链更清晰
- 隐式副作用更少

## 2. 一个最小 Go 项目应该长什么样

最小项目：

```text
demo/
├── go.mod
└── main.go
```

初始化模块：

```bash
go mod init demo
```

示例代码：

```go
package main

import "fmt"

func main() {
	fmt.Println("hello, go")
}
```

运行：

```bash
go run .
```

构建：

```bash
go build .
```

这里最关键的概念不是命令，而是：

- 一个目录通常对应一个 package
- 一个项目通过 `go.mod` 定义模块边界
- import 路径基于 module path + package path

## 3. 基础语法里最值得真正搞懂的点

### 变量声明

```go
var name string = "sky"
age := 18
```

经验规则：

- 局部作用域优先用 `:=`
- 包级变量慎用，尤其是可变状态
- 能就地初始化就不要拖到后面

### `if`、`for`、`switch`

Go 没有 `while`，统一用 `for`。

```go
for i := 0; i < 10; i++ {
}

for condition() {
}

for {
	break
}
```

`switch` 默认不会像 C 那样自动穿透，除非你显式写 `fallthrough`。

这意味着：

- 大多数 `switch` 更安全
- 不需要为了防穿透写一堆 `break`

### `defer`

`defer` 非常适合处理成对资源：

- 文件关闭
- 锁释放
- `cancel()` 调用
- 埋点收尾

例如：

```go
f, err := os.Open("data.txt")
if err != nil {
	return err
}
defer f.Close()
```

但要知道两点：

1. `defer` 参数会在声明时求值。
2. 在极端热点循环里大量使用 `defer` 需要关注开销。

## 4. 数据类型：Go 的“常用地基”

### 数组、切片、map

数组长度是类型的一部分：

```go
var a [3]int
```

切片是最常用的序列结构：

```go
s := []int{1, 2, 3}
s = append(s, 4)
```

理解切片时，必须同时理解：

- 指向底层数组的指针
- `len`
- `cap`

否则就很容易在扩容、共享底层数组、子切片修改时踩坑。

相关专题见：

- [Go slice 避坑指南](./go-slice-pitfalls.md)
- [Go map 避坑指南](./go-map-pitfalls.md)

### `string`、`[]byte`、`rune`

Go 中：

- `string` 是只读字节序列
- `[]byte` 适合二进制与 I/O
- `rune` 表示一个 Unicode code point

这几个概念如果混在一起，就很容易在中文、编码、截断、网络传输时出问题。

经验规则：

- 文本处理优先搞清楚是“字节长度”还是“字符长度”
- I/O、网络、序列化通常用 `[]byte`
- 需要按字符遍历时考虑 `rune`

### `struct`

Go 里 `struct` 是最核心的数据组织方式。

```go
type User struct {
	ID   int64
	Name string
}
```

好的 `struct` 设计原则：

- 字段职责清晰
- 避免一个结构体承载太多层含义
- 不要把 DTO、数据库模型、领域对象、接口响应全混成一个结构体

## 5. 方法、接收者、组合

Go 没有 class，但有方法。

```go
type Counter struct {
	n int
}

func (c *Counter) Inc() {
	c.n++
}
```

### 值接收者还是指针接收者

经验规则：

- 需要修改状态时，用指针接收者
- 结构体较大时，通常也用指针接收者
- 如果一个类型的大多数方法是指针接收者，最好保持一致

相关专题见：

- [Go pointer 避坑指南](./go-pointer-pitfalls.md)

### 组合优先于继承

Go 常见写法是嵌入或持有另一个类型：

```go
type Logger struct{}

type Service struct {
	logger Logger
}
```

或者：

```go
type Base struct{}

type UserService struct {
	Base
}
```

但要注意：嵌入不等于“面向对象继承体系”。它更像是字段提升和能力复用。

## 6. 接口：Go 最重要也最容易滥用的抽象

Go 接口的设计哲学是“小而精”。

例如标准库里的：

```go
type Reader interface {
	Read(p []byte) (n int, err error)
}
```

### 隐式实现

一个类型只要实现了接口的方法集合，就自动实现该接口，不需要显式声明。

这带来两个重要结果：

1. 解耦很自然。
2. 也更容易在接口设计过大时失控。

### 什么时候该定义接口

一个很实用的规则是：

- 接口应由使用方定义，而不是实现方抢先定义

因为调用者才真正知道“它需要的最小能力集合”是什么。

### 不要为了“可测试”先造一堆大接口

很多 Go 新手会写出这种代码：

```go
type UserServiceInterface interface {
	CreateUser(ctx context.Context, req CreateUserRequest) (*User, error)
	UpdateUser(ctx context.Context, req UpdateUserRequest) error
	DeleteUser(ctx context.Context, id int64) error
	GetUser(ctx context.Context, id int64) (*User, error)
	ListUsers(ctx context.Context, req ListUsersRequest) ([]User, error)
}
```

如果只有一个实现者、一个消费者、一个服务对象，这种接口通常过早抽象了。

相关专题见：

- [Go interface 避坑指南](./go-interface-pitfalls.md)

## 7. 泛型：能用，但不要滥用

Go 1.18 之后有了泛型，但 Go 对泛型的态度依然偏克制。

泛型最适合的场景：

- 通用数据结构
- 通用算法
- 明显重复、仅类型不同的逻辑

例如：

```go
func First[T any](items []T) (T, bool) {
	var zero T
	if len(items) == 0 {
		return zero, false
	}
	return items[0], true
}
```

不太建议的场景：

- 为了“优雅”把业务逻辑泛型化
- 团队还没形成统一风格就大量引入复杂约束

经验规则：

- 先写清晰的普通代码
- 只有在抽象收益非常明显时再用泛型

## 8. 错误处理：Go 项目质量的分水岭

Go 错误处理的主路径是返回 `error`：

```go
func parse(input string) error {
	if input == "" {
		return errors.New("empty input")
	}
	return nil
}
```

### 包装错误

推荐用 `%w` 包装：

```go
if err != nil {
	return fmt.Errorf("load config: %w", err)
}
```

这样上层可以继续用 `errors.Is`、`errors.As` 判断根因。

### 什么时候用 `panic`

`panic` 通常只适合：

- 真正不可恢复的程序错误
- 初始化阶段的致命问题
- 框架边界处统一兜底恢复

业务错误、校验错误、数据库错误，大多数都不应该用 `panic`。

相关专题见：

- [Go error 避坑指南](./go-error-pitfalls.md)

## 9. 标准库里最值得熟练掌握的模块

Go 写项目时，标准库比很多人想象得更重要。

优先掌握这些：

- `context`
- `errors`
- `fmt`
- `time`
- `strings`
- `bytes`
- `strconv`
- `sync`
- `sync/atomic`
- `net/http`
- `encoding/json`
- `database/sql`
- `os`
- `io`
- `bufio`
- `testing`

一个常见误区是：一上来先找第三方库，而不是先确认标准库能否已经满足需求。

## 10. 日常开发里最常见的 Go 使用姿势

### JSON 编解码

```go
type CreateUserReq struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}
```

需要关注：

- 字段是否导出
- tag 是否正确
- 空值语义是否明确
- `omitempty` 是否会导致语义丢失

### 时间处理

Go 的时间格式不是 `YYYY-MM-DD` 这种占位符，而是基于参考时间：

```go
time.Now().Format("2006-01-02 15:04:05")
```

### 上下文传递

服务端代码通常都应该显式传 `ctx`：

```go
func (s *Service) CreateUser(ctx context.Context, req CreateUserReq) error
```

相关专题见：

- [Go context 避坑指南](./go-context-pitfalls.md)

## 11. 初学 Go 最容易踩的几个“思维坑”

### 把 Go 写成面向对象大工程

表现为：

- 层数过多
- 接口满天飞
- getter / setter 成堆
- 代码看似规范，实则移动很慢

Go 更适合清晰、扁平、面向组合的设计。

### 过度追求“高级技巧”

表现为：

- 泛型过度抽象
- 反射滥用
- 框架封装太厚
- 动不动造 DSL

Go 的强项不在这些地方，而在简单直接、可维护、易调试。

### 忽略底层数据结构行为

例如：

- 不理解切片扩容
- 不理解 map 读取零值
- 不理解接口里的动态类型和值
- 不理解指针和值语义

这类问题往往不会立刻报错，但会在边界场景和线上高并发时暴露。

## 12. 学 Go 的建议顺序

推荐顺序：

1. 先把 package、函数、结构体、方法、接口、error 搞清楚
2. 再把 slice、map、pointer、context 真正理解透
3. 再学 goroutine、channel、`sync`
4. 再进入工程化、Web、数据库、RPC
5. 最后再补性能优化、代码生成、微服务治理

很多人一上来就学 Gin、学微服务框架，但基础心智没建立，后面遇到问题很难定位根因。

## 13. 和这份文档配套的专题文档

如果你看完这篇，建议继续读：

- [Go 并发模型与 Runtime 心智模型](./go-runtime-concurrency.md)
- [Go 项目工程化与后端实践](./go-project-engineering.md)
- [Gin 框架实战指南](./go-gin-guide.md)
- [go-zero 架构与实战认知](./go-zero-guide.md)
- [gRPC 通信协议深度解读](./go-grpc-deep-dive.md)

## 14. 一句话总结

Go 语言真正的学习重点不是“关键字列表”，而是：

- 用简单的数据结构表达问题
- 用显式的数据流组织程序
- 用小接口和组合控制复杂度
- 用错误返回、上下文、并发原语写出可维护的服务
