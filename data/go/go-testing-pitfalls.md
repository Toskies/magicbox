# Go testing 避坑指南

Go 自带的 `testing` 包很简洁，但测试写不好，问题通常不是“跑不起来”，而是：

- 测试不稳定
- 测试互相污染
- 失败信息没法定位
- 看似覆盖了，其实根本没测到关键行为

这份文档重点讲 Go 测试里的高频避坑点。

## 1. 测试名太模糊，失败后几乎没有定位价值

错误示例：

```go
func Test1(t *testing.T) {}
func TestWork(t *testing.T) {}
```

这种命名在测试多了以后几乎没有信息量。

更好的方式是把测试名写成行为描述：

```go
func TestParseUser_ReturnsErrorWhenIDMissing(t *testing.T) {}
```

原则：

- 名字写“场景 + 预期行为”
- 失败时能靠测试名快速判断意图

## 2. 表驱动测试写了，但失败信息太差

错误示例：

```go
for _, tc := range tests {
	got := add(tc.a, tc.b)
	if got != tc.want {
		t.Fail()
	}
}
```

这类失败输出通常只告诉你“挂了”，却不知道是哪组 case 挂、输入是什么、实际值是什么。

更稳妥的写法：

```go
for _, tc := range tests {
	got := add(tc.a, tc.b)
	if got != tc.want {
		t.Fatalf("a=%d b=%d got=%d want=%d", tc.a, tc.b, got, tc.want)
	}
}
```

## 3. 子测试没命名，输出不利于排查

推荐写法：

```go
for _, tc := range tests {
	t.Run(tc.name, func(t *testing.T) {
		got := add(tc.a, tc.b)
		if got != tc.want {
			t.Fatalf("got=%d want=%d", got, tc.want)
		}
	})
}
```

这样：

- `-run` 可以精确筛 case
- 报错信息更容易定位
- 输出更结构化

## 4. 子测试里共享可变状态，容易互相污染

错误示例：

```go
store := NewStore()

for _, tc := range tests {
	t.Run(tc.name, func(t *testing.T) {
		store.Add(tc.input)
	})
}
```

如果每个 case 都复用同一个可变对象，后面的 case 很可能受到前面 case 的影响。

避坑原则：

- 每个 case 尽量独立构造输入和依赖
- 除非你明确在测试“累积行为”，否则不要共享可变状态

## 5. `t.Parallel()` 用得太随意，测试会变得很脆弱

`t.Parallel()` 很有用，但只适合真正互不影响的测试。

不适合直接并行的场景：

- 共享全局变量
- 共享临时目录或固定文件名
- 依赖环境变量
- 依赖当前时间、端口、数据库状态

错误倾向：

```go
func TestA(t *testing.T) {
	t.Parallel()
	os.Setenv("MODE", "A")
}
```

如果多个测试并行改环境变量，结果通常非常不稳定。

## 6. 该用 `t.Fatalf` 的地方用了 `t.Error`

如果后续逻辑依赖前面的检查结果成立，就不应该继续往下跑。

例如：

```go
if err != nil {
	t.Error(err)
}
got := result.Value
```

如果 `err != nil` 时 `result` 已经无效，继续执行只会制造二次噪音。

更合适：

```go
if err != nil {
	t.Fatalf("unexpected err: %v", err)
}
```

## 7. 断言过度粗糙，错误信息没法读

错误示例：

```go
if !reflect.DeepEqual(got, want) {
	t.Fail()
}
```

更稳妥的写法至少应该打印两边值：

```go
if !reflect.DeepEqual(got, want) {
	t.Fatalf("got=%v want=%v", got, want)
}
```

如果对象复杂，建议引入更易读的 diff 工具，或者把比较拆细。

## 8. 测试依赖真实时间，容易不稳定

错误示例：

```go
time.Sleep(100 * time.Millisecond)
if !done {
	t.Fatal("not done")
}
```

这类测试在不同机器、CI 负载、调度状态下很容易波动。

更好的方式通常是：

- 注入时钟
- 使用同步信号
- 明确等待条件
- 给超时留足安全边界

不要把 `Sleep` 当作默认同步手段。

## 9. 测试依赖随机数，但没固定 seed

错误示例：

```go
r := rand.New(rand.NewSource(time.Now().UnixNano()))
```

这样测试一旦失败，很难稳定复现。

如果你需要随机性测试，建议：

- 固定 seed，保证可复现
- 失败时打印 seed
- 把随机测试和确定性测试分层

## 10. 忽略清理逻辑，临时资源会污染后续测试

常见场景：

- 临时文件没删
- goroutine 没收尾
- 数据库记录没清理
- 环境变量没恢复

推荐做法：

```go
dir := t.TempDir()
```

以及：

```go
t.Cleanup(func() {
	// restore / close / cleanup
})
```

让清理逻辑和测试绑定，而不是靠人工记忆。

## 11. 全局状态不隔离，测试很容易串

错误示例：

```go
var mode = "prod"

func TestA(t *testing.T) {
	mode = "test"
}
```

如果其他测试也依赖这个全局状态，执行顺序一变就可能炸。

避坑原则：

- 少依赖全局状态
- 必须改时要在测试内恢复
- 更好的方式是依赖注入

## 12. Benchmark 写法不对，测到的不是你想测的内容

错误示例：

```go
func BenchmarkParse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		input := buildLargeInput()
		parse(input)
	}
}
```

如果你本来想测 `parse`，但每轮都把构造输入算进去了，结果就被污染了。

更稳妥的做法：

- 预先准备输入
- 在循环里只测目标逻辑
- 必要时用 `b.ResetTimer()`

## 13. `go test` 缓存会干扰排查

很多人改了外部依赖或测试环境后，发现 `go test` 看起来行为不对，结果其实是命中了缓存。

排查时常用：

```bash
go test -count=1 ./...
```

这样可以强制重新执行。

## 14. `t.Helper()` 不加，错误栈会不好读

如果你封装了测试辅助函数：

```go
func mustEqual(t *testing.T, got, want any) {
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
}
```

最好加上：

```go
t.Helper()
```

这样报错位置会指向调用方，而不是辅助函数内部。

## 15. 常见安全写法

### 表驱动 + 子测试

```go
for _, tc := range tests {
	t.Run(tc.name, func(t *testing.T) {
		got := fn(tc.input)
		if got != tc.want {
			t.Fatalf("got=%v want=%v", got, tc.want)
		}
	})
}
```

### 自动清理资源

```go
dir := t.TempDir()
t.Cleanup(func() {
	_ = dir
})
```

### 强制重新跑测试

```bash
go test -count=1 ./...
```

### 只跑某个测试

```bash
go test -run TestParseUser -v ./...
```

## 16. 实战记忆法

看到测试代码时，可以先问自己：

1. 这个测试失败时，信息够不够定位问题？
2. 每个 case 是否相互独立？
3. `t.Parallel()` 会不会引入共享状态冲突？
4. 有没有依赖时间、随机数、环境变量这类不稳定因素？
5. 清理逻辑是不是自动完成？
6. Benchmark 到底测的是目标逻辑，还是把准备成本也算进去了？

## 17. 一份简短的避坑清单

- 测试名是否足够表达场景和意图？
- 表驱动失败信息是否包含输入/输出？
- 子测试是否共享了可变状态？
- `t.Parallel()` 是否真的安全？
- 是否依赖随机数、时间或环境导致不稳定？
- 是否使用了 `TempDir`、`Cleanup`、`Helper` 提升可维护性？

如果这些问题都提前想清楚，Go 测试里的大多数坑都能避开。
