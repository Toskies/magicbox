# Go 性能分析与 Profiling 方法

Go 的性能问题如果只靠“感觉”，通常会越改越乱。

真正有效的方式是：

1. 先测
2. 再看 profile
3. 确认热点
4. 最后再优化

这份文档重点讲 Go 里最常用的性能分析手段，包括：

- benchmark
- CPU profile
- memory profile
- goroutine / block / mutex profile
- trace
- 堆栈和热点定位

## 1. 先确认你是在测什么

性能分析最常见的问题不是不会用工具，而是测错对象。

例如：

- 你以为在测解析逻辑，其实把构造输入时间也算进去了
- 你以为在测业务代码，其实大头都耗在网络 I/O
- 你以为在测单函数，实际上锁争用才是主因

所以第一步不是跑 `pprof`，而是先明确：

- 要测吞吐、延迟，还是内存？
- 是测单函数、单接口，还是整条链路？
- 是本地纯 CPU 问题，还是外部依赖导致？

## 2. 先写 benchmark，再谈优化

Go 的 benchmark 入口：

```go
func BenchmarkParse(b *testing.B) {
	input := prepareInput()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parse(input)
	}
}
```

常用命令：

```bash
go test -bench . ./...
go test -bench . -benchmem ./...
```

其中：

- `-bench .` 跑所有 benchmark
- `-benchmem` 额外输出分配次数和内存大小

如果优化目标不清楚，先写 benchmark，避免“优化后反而变差还没发现”。

## 3. CPU profile：先看 CPU 时间花在哪

生成 CPU profile 的常见方式：

```bash
go test -bench . -cpuprofile cpu.out ./...
```

查看：

```bash
go tool pprof cpu.out
```

进入交互后常用命令：

- `top` 看热点函数
- `top -cum` 看累计耗时
- `list FuncName` 看某函数源码热点
- `web` 生成调用图

CPU profile 适合回答的问题：

- 时间主要耗在哪个函数
- 是自己代码热，还是标准库/JSON/正则/排序热
- 是算法问题，还是字符串/内存分配问题带来的 CPU 消耗

## 4. Memory profile：看分配，而不只是看占用

生成方式：

```bash
go test -bench . -memprofile mem.out ./...
```

查看：

```bash
go tool pprof mem.out
```

内存分析时要注意一个核心点：

- 不只是看“最后占了多少内存”
- 更要看“哪里在大量分配”

很多性能问题的根因其实是：

- 频繁分配对象
- 字符串拼接过多
- 切片/map 扩容过多
- 临时对象太多导致 GC 压力大

## 5. `-benchmem` 是第一手低成本信号

很多时候你都不用先开 profile，只看 benchmark 输出就能发现问题。

例如：

```text
1000000	1200 ns/op	512 B/op	8 allocs/op
```

其中：

- `ns/op` 表示每次操作耗时
- `B/op` 表示每次操作分配的字节数
- `allocs/op` 表示每次操作分配次数

如果你优化前后 `allocs/op` 明显下降，通常就是一个非常强的信号。

## 6. goroutine profile：查谁没退出、谁在堆积

如果你怀疑：

- goroutine 泄漏
- 服务长时间运行后 goroutine 越来越多
- 某类 worker 没有退出

可以看 goroutine profile。

常见服务里会通过 `net/http/pprof` 暴露：

```go
import _ "net/http/pprof"
```

然后访问：

```text
/debug/pprof/goroutine
```

你要看的是：

- 哪类堆栈重复出现很多次
- 大家都卡在 channel、锁、IO 还是 select 上

## 7. block profile：查阻塞等待

如果系统表现为：

- CPU 不高
- 吞吐却上不去
- 看起来“慢但不忙”

很可能是大量时间耗在阻塞等待上。

这时候 block profile 很有价值。

启用通常需要在程序里配置：

```go
runtime.SetBlockProfileRate(1)
```

然后通过 pprof 查看 block profile。

适合定位：

- channel 等待
- select 阻塞
- 某些同步点等待过长

## 8. mutex profile：查锁竞争

如果你怀疑：

- CPU 不低但吞吐很差
- 多 goroutine 并发时性能明显掉
- 某段代码串行化严重

可以打开 mutex profile。

常见方式：

```go
runtime.SetMutexProfileFraction(1)
```

然后再通过 pprof 分析 mutex profile。

它能帮助你看：

- 哪些锁竞争最严重
- 时间主要耗在等哪把锁

## 9. trace：看时序、调度、阻塞，比 pprof 更像“动态录像” 

如果 pprof 解决的是“热在哪”，`trace` 更像是在回答：

- goroutine 是怎么调度的
- 哪些阶段在阻塞
- 网络、syscall、GC、调度切换发生了什么

常见生成方式：

```bash
go test -run '^$' -bench BenchmarkFoo -trace trace.out ./...
```

查看：

```bash
go tool trace trace.out
```

`trace` 特别适合：

- 并发程序时序问题
- 某些阶段突然卡顿
- scheduler 行为分析
- GC 和业务执行互相影响

## 10. 用 `list` 把热点定位回源码

很多人只看 `top`，但真正落到代码改动时，更有价值的是：

```bash
go tool pprof cpu.out
(pprof) list ParseUser
```

这样可以直接看到函数里哪些行消耗高。

这比只看函数名更能指导真实优化。

## 11. Web 图和火焰图适合看调用关系，不适合替代思考

`pprof` 的图形化视图很直观，但要注意：

- 图适合看调用链结构
- `top` 和 `list` 更适合看具体热点

不要只看图感受“哪块红”，而不去确认：

- 它是自身耗时高
- 还是只是被大量调用导致累计时间高

这就是为什么：

- `flat`
- `cum`

都要一起看。

## 12. 堆栈转储适合查卡死和泄漏，不只是查 panic

如果服务看起来卡住、请求挂起、goroutine 很多，可以先拿堆栈。

常见方式：

- `kill -QUIT <pid>`
- goroutine pprof
- trace 里的 goroutine 分析

重点看：

- 是否很多 goroutine 卡在同一位置
- 是否大量等锁、等 channel、等网络
- 是否存在明显的 worker 不退出模式

## 13. 性能分析时要区分“热点”和“根因”

例如你看到：

- `runtime.mallocgc` 很热

这通常不是说“runtime 有问题”，而是说明：

- 你的代码分配太多对象

你看到：

- `sync.(*Mutex).Lock` 很热

也不代表“锁实现有问题”，而是：

- 你的业务在争同一把锁

所以 profile 里出现的热点函数，很多时候只是“症状出现的位置”，还需要再往上追业务根因。

## 14. 优化前后要可比较

不要“改了一堆，然后感觉快了”。

正确方式应该是：

1. 先基线 benchmark/profile
2. 做单一改动
3. 再跑同样的 benchmark/profile
4. 对比 `ns/op`、`allocs/op`、热点函数变化

只有这样，优化才是可验证的。

## 15. 常用命令清单

### benchmark

```bash
go test -bench . ./...
go test -bench . -benchmem ./...
```

### CPU profile

```bash
go test -bench . -cpuprofile cpu.out ./...
go tool pprof cpu.out
```

### memory profile

```bash
go test -bench . -memprofile mem.out ./...
go tool pprof mem.out
```

### trace

```bash
go test -run '^$' -bench BenchmarkFoo -trace trace.out ./...
go tool trace trace.out
```

### 交互式 pprof 常用命令

```text
top
top -cum
list FuncName
web
```

## 16. 一份推荐使用顺序

遇到性能问题时，可以先按这个顺序来：

1. 先写 benchmark 或确定稳定压测场景。
2. 先看 `-benchmem`，确认是否分配过多。
3. CPU 慢就先抓 `cpuprofile`。
4. 内存/GC 压力大就抓 `memprofile`。
5. 怀疑调度、阻塞、并发时序问题，就上 `trace`。
6. 怀疑锁竞争，就看 mutex profile。
7. 怀疑 goroutine 泄漏或卡死，就看 goroutine stack/profile。
8. 用 `list` 把热点落回源码，再做单点优化。

## 17. 一份简短的 profiling 清单

- benchmark 是否测准了目标逻辑？
- 是否有优化前基线数据？
- `ns/op`、`B/op`、`allocs/op` 分别说明了什么？
- 热点是在自己代码，还是 runtime/库函数？
- 是 CPU 热、分配多、锁竞争，还是阻塞等待？
- 优化后是否用同样命令重新验证过？

如果按这套方式来，Go 性能分析基本就不会停留在“凭感觉调优”。
