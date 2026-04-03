# debug 示例目录

这个目录把 [go-debugging-workflow.md](../code-doc/go-debugging-workflow.md) 第 6 条开始提到的调试手法落成了可运行示例，重点覆盖：

- `go test` 精准调试
- `-race` 并发问题排查
- `dlv` 断点调试
- goroutine 栈与 `pprof` / `trace`
- benchmark 与 `-benchmem`

## 目录说明

- `workflow.go`
  提供可调试的业务代码：解析用户记录、慢版/快版摘要构建、非线程安全和线程安全计数器。
- `workflow_test.go`
  提供测试、子测试、race 示例、artifact 生成测试和 benchmark。
- `diagnostics.go`
  提供栈、goroutine profile、heap profile、cpu profile、trace 的生成函数。
- `cmd/dlvdemo/main.go`
  提供可直接给 `dlv debug` 使用的主程序。

## 对应文档第 6 条：常用 `go test` 调试命令

只跑一个测试：

```bash
go test -run TestParseUserRecord -v ./debug
```

只跑某个子测试：

```bash
go test -run 'TestParseUserRecord/invalid_id' -v ./debug
```

禁用缓存强制重跑：

```bash
go test -count=1 ./debug
```

重复跑同一个测试：

```bash
go test -run TestParseUserRecord -count=50 -v ./debug
```

## 对应文档第 7 条：并发问题优先跑 `-race`

这个目录里 `TestUnsafeCounterRace` 会在开启环境变量后故意触发共享变量竞争：

```bash
DEBUG_RUN_RACE_EXAMPLE=1 go test -race -run TestUnsafeCounterRace -v ./debug
```

安全版本可以直接对照跑：

```bash
go test -run TestLockedCounter -v ./debug
```

## 对应文档第 8 条：用 Delve 做断点调试

调试这个目录里的主程序：

```bash
dlv debug ./debug/cmd/dlvdemo
```

建议尝试的命令：

```text
b main.main
b debug.ParseUserRecord
c
n
s
p users
bt
```

调试测试：

```bash
dlv test ./debug -- -test.run TestParseUserRecord/invalid_id
```

## 对应文档第 9 条：看 goroutine 栈和 profile

运行下面的命令会把调试产物写到 `debug/out`：

```bash
go run ./debug/cmd/dlvdemo -artifacts-dir ./debug/out
```

生成后可以直接查看：

```bash
cat debug/out/goroutine-stacks.txt
go tool pprof debug/out/cpu.prof
go tool pprof debug/out/heap.prof
go tool pprof debug/out/goroutine.prof
go tool trace debug/out/trace.out
```

如果更想直接在浏览器里看 profile，也可以这样打开：

```bash
go tool pprof -http=:6060 ./debug/out/cpu.prof
go tool pprof -http=:6060 ./debug/out/heap.prof
go tool pprof -http=:6060 ./debug/out/goroutine.prof
go tool trace -http=:6061 ./debug/out/trace.out
```

常见习惯是：

- `cpu.prof` 看 CPU 热点和调用图
- `heap.prof` 看内存分配热点
- `goroutine.prof` 看 goroutine 堆积和阻塞位置
- `trace.out` 看调度、阻塞、GC 和时序

如果端口被占用，把 `6060` 或 `6061` 改成别的本地端口即可。

如果只想通过测试生成一次临时产物，也可以跑：

```bash
go test -run TestWriteDebugArtifacts -v ./debug
```

## 对应文档第 12 条：benchmark 与性能分析

跑 benchmark：

```bash
go test -bench BenchmarkBuildSummary -benchmem ./debug
```

只测快版本并输出 CPU profile：

```bash
go test -run ^$ -bench BenchmarkBuildSummaryFast -cpuprofile cpu.out ./debug
```

只测快版本并输出内存 profile：

```bash
go test -run ^$ -bench BenchmarkBuildSummaryFast -memprofile mem.out ./debug
```

随后查看：

```bash
go tool pprof cpu.out
go tool pprof mem.out
go tool pprof -http=:6060 cpu.out
go tool pprof -http=:6060 mem.out
```

## 建议的练习顺序

1. 先跑 `TestParseUserRecord` 和它的子测试，熟悉 `-run`。
2. 再跑 `TestLockedCounter` 和 `TestUnsafeCounterRace`，体会 `-race` 的价值。
3. 然后用 `dlv debug` 单步跟进 `ParseUserRecord`。
4. 最后跑 benchmark 和 profile，比较 `BuildSummarySlow` / `BuildSummaryFast` 的差异。
