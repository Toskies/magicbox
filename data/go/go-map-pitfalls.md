# Go map 避坑指南

`map` 是 Go 里最常用的数据结构之一，但它的行为和很多语言里的哈希表并不完全一样。

很多 bug 不是出在不会用，而是出在“以为它会像别的语言那样工作”。尤其是下面这些点，最容易踩坑：

- 读取不存在的 key 时不会报错，而是返回零值。
- `nil map` 可以读，但不能写。
- `map` 遍历顺序不稳定，不能依赖顺序。
- 并发读写普通 `map` 会出问题。
- `map` 元素不可寻址，不能直接改字段。
- 边遍历边修改时，逻辑容易失控。

这份文档重点讲 `map` 的高频避坑点和实战写法。

## 1. 读取不存在的 key，不会报错

示例：

```go
m := map[string]int{
	"a": 1,
}

fmt.Println(m["a"]) // 1
fmt.Println(m["b"]) // 0
```

`m["b"]` 不会 panic，也不会报错，而是直接返回 value 类型的零值。

这很容易带来歧义：

- 对 `map[string]int` 来说，不存在时返回 `0`
- 对 `map[string]bool` 来说，不存在时返回 `false`
- 对 `map[string]*User` 来说，不存在时返回 `nil`

如果零值本身就是合法值，就不能只靠 `m[key]` 判断 key 是否存在。

正确写法：

```go
v, ok := m["b"]
if !ok {
	fmt.Println("key 不存在")
	return
}

fmt.Println(v)
```

记住一句话：

> 只要“零值”和“key 不存在”有歧义，就一定要用 `value, ok := m[key]`。

## 2. `nil map` 可以读，但不能写

这是 `map` 最经典的坑之一。

示例：

```go
var m map[string]int

fmt.Println(m["x"]) // 0
m["x"] = 1         // panic
```

很多人第一次遇到会困惑：

- 为什么读没问题？
- 为什么写就 panic？

原因是 `nil map` 本质上还没有真正分配哈希表结构。读取时 Go 直接返回零值；写入时因为没有可写入的存储空间，所以会 panic。

正确写法：

```go
m := make(map[string]int)
m["x"] = 1
```

如果你拿到的是可能为 `nil` 的 map，写入前要先初始化：

```go
if m == nil {
	m = make(map[string]int)
}
```

## 3. `map` 遍历顺序不能依赖

错误示例：

```go
m := map[string]int{
	"a": 1,
	"b": 2,
	"c": 3,
}

for k := range m {
	fmt.Println(k)
}
```

输出顺序不是固定的，不能假设它一定是 `a -> b -> c`。

避坑原则：

- `map` 是无序的。
- 遍历顺序不要作为业务逻辑前提。
- 测试里也不要直接断言 `range map` 的顺序。

如果你需要稳定顺序，正确做法是先取 key，再排序：

```go
keys := make([]string, 0, len(m))
for k := range m {
	keys = append(keys, k)
}

sort.Strings(keys)

for _, k := range keys {
	fmt.Println(k, m[k])
}
```

## 4. `map` 元素不可取地址，也不能直接改字段

这是 Go 新手非常容易踩的编译期错误。

错误示例：

```go
type User struct {
	Name string
}

m := map[string]User{
	"u1": {Name: "Tom"},
}

m["u1"].Name = "Jerry" // 编译错误
```

为什么不行：

- `map` 内部在扩容和搬迁时，元素位置可能变化。
- Go 不允许你直接拿 `map` 元素的地址来做原地修改。

正确做法有两种。

### 写法一：取出，修改，再放回去

```go
u := m["u1"]
u.Name = "Jerry"
m["u1"] = u
```

### 写法二：value 改成指针

```go
m := map[string]*User{
	"u1": &User{Name: "Tom"},
}

m["u1"].Name = "Jerry"
```

什么时候用哪种：

- 数据小、希望值语义清晰，用“取出再写回”。
- 对象较大、频繁修改字段，可以考虑存指针。

## 5. `map[string]T` 和 `map[string]*T` 的语义差别很大

这不是语法问题，而是设计问题。

### `map[string]T`

特点：

- 读出来的是值拷贝。
- 改字段后不会自动写回。
- 语义更明确，不容易共享可变状态。

### `map[string]*T`

特点：

- 读出来是指针。
- 改字段会直接影响原对象。
- 容易共享状态，也更容易引入并发问题和空指针问题。

错误示例：

```go
type User struct {
	Name string
}

m := map[string]*User{}
m["u1"].Name = "Tom" // panic，m["u1"] 是 nil
```

如果 value 是指针类型，访问前要确认对象已经初始化。

## 6. 并发读写普通 `map` 是危险的

这是生产环境里很常见的问题。

错误示例：

```go
m := map[string]int{}

go func() {
	for {
		m["x"]++
	}
}()

go func() {
	for {
		_ = m["x"]
	}
}()
```

这种代码可能直接触发运行时错误：

```text
fatal error: concurrent map read and map write
```

避坑原则：

- 普通 `map` 不是并发安全的。
- 多 goroutine 并发访问时，要么加锁，要么换成并发安全方案。

常见做法：

### 加 `sync.RWMutex`

```go
type Counter struct {
	mu sync.RWMutex
	m  map[string]int
}

func (c *Counter) Get(key string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.m[key]
}

func (c *Counter) Inc(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key]++
}
```

### 使用 `sync.Map`

适合读多写少、key 动态、并发场景明确的情况，但不要把它当作普通 `map` 的无脑替代。

## 7. 边遍历边删除，虽然能做，但要清楚影响

示例：

```go
for k, v := range m {
	if v == 0 {
		delete(m, k)
	}
}
```

这类写法在 Go 里是允许的，删除当前或尚未遍历到的元素，不会像某些语言那样直接抛异常。

但要注意两点：

- 遍历顺序本来就是不稳定的。
- 一边遍历一边新增元素，哪些新元素会不会被遍历到，并不适合作为逻辑依赖。

建议：

- 删除可以在遍历时做。
- 新增、迁移、复杂重建逻辑，最好拆成两步。

更稳妥的写法：

```go
keysToDelete := make([]string, 0)
for k, v := range m {
	if v == 0 {
		keysToDelete = append(keysToDelete, k)
	}
}

for _, k := range keysToDelete {
	delete(m, k)
}
```

## 8. 判断 key 是否存在，不要用 value 和零值硬猜

错误示例：

```go
online := map[string]bool{}

if !online["tom"] {
	fmt.Println("tom 不在线")
}
```

这段代码的问题是：

- `tom` 不存在，返回 `false`
- `tom` 存在但值就是 `false`，也返回 `false`

两种情况被混在一起了。

正确写法：

```go
v, ok := online["tom"]
if !ok {
	fmt.Println("tom 这个 key 不存在")
	return
}

if !v {
	fmt.Println("tom 存在，但状态是 false")
}
```

## 9. 不要指望 `map` 自动深拷贝

示例：

```go
m1 := map[string][]int{
	"a": {1, 2, 3},
}

m2 := make(map[string][]int, len(m1))
for k, v := range m1 {
	m2[k] = v
}

m2["a"][0] = 99
fmt.Println(m1["a"]) // [99 2 3]
```

这里虽然新建了 `map`，但 value 是切片，而切片本身又共享底层数组，所以这只是浅拷贝。

如果 value 是引用语义类型：

- `[]T`
- `map[K]V`
- `*T`

就要警惕“map 拷贝了，但内部数据其实还共享”。

真正需要独立副本时，要继续对 value 做深拷贝。

## 10. `delete` 不会报错，也不会返回是否删除成功

示例：

```go
delete(m, "not-exist")
```

即使 key 不存在，也不会 panic。

这通常是好事，但也意味着：

- 如果你需要区分“本来有然后删掉了”还是“原本就没有”，要自己先判断。

例如：

```go
if _, ok := m[key]; ok {
	delete(m, key)
}
```

## 11. `clear` 会清空 map，但不会改变它是否为 `nil`

Go 提供了 `clear`：

```go
clear(m)
```

它会删除 map 中所有元素。

但要注意：

- `clear(m)` 之后，`m` 还是一个已初始化的 map，不是 `nil`
- 如果本来是 `nil map`，`clear(nilMap)` 也是安全的

如果你的逻辑里需要区分“空 map”和“未初始化 map”，就不能把 `clear` 当作“恢复到 nil”。

## 12. 常见安全写法

### 安全读取

```go
v, ok := m[key]
if !ok {
	// 不存在
}
```

### 初始化后再写入

```go
if m == nil {
	m = make(map[string]int)
}
m[key] = 1
```

### 稳定顺序输出

```go
keys := make([]string, 0, len(m))
for k := range m {
	keys = append(keys, k)
}
sort.Strings(keys)
```

### 修改结构体 value

```go
u := m[id]
u.Name = "new"
m[id] = u
```

### 并发访问时加锁

```go
mu.Lock()
m[key] = value
mu.Unlock()
```

## 13. 实战记忆法

写 `map` 相关代码时，可以先问自己：

1. 这里读取不到 key 时，零值会不会造成歧义？
2. 这个 map 有没有可能还是 `nil`？
3. 我是不是在依赖 `range map` 的顺序？
4. 这里有没有多 goroutine 同时访问？
5. 我是不是在直接修改 `map` value 的字段？
6. 这个“拷贝 map”的动作，是不是其实只做了浅拷贝？

## 14. 一份简短的避坑清单

- 读取 key 时，是否需要 `value, ok := m[key]`？
- 写入前，map 是否已经 `make` 过？
- 是否错误依赖了遍历顺序？
- 是否有并发读写风险？
- 是否错误地直接修改了 `map` 元素字段？
- 当前复制的是 map 本身，还是连 value 也一起深拷贝了？

如果这些问题都想清楚，Go `map` 的大多数坑都能避开。
