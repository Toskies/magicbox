# Go 知识地图

`data/go` 这一类内容，建议分成两层来看：

1. 一层是“体系化学习文档”，帮助你建立 Go 的整体认知。
2. 一层是“专题避坑与排障文档”，帮助你在真实项目里少踩坑、快定位。

如果你是从零开始补 Go 知识，推荐先读体系化文档；如果你已经在写 Go 服务，可以把后面的避坑文档当成工作台手册。

## 全量目录速览

当前这一分类下的文档可以按 4 组来理解：

- 学习主线：从语言基础、并发模型、锁源码、工程化、依赖管理，一路走到 Gin、go-zero、gRPC。
- 基础心智与高频易错点：围绕 `slice`、`map`、指针、接口、错误、`context`、goroutine、channel、锁等主题。
- 框架与服务化专题：围绕 Gin、go-zero、gRPC、项目结构与依赖管理。
- 排障与性能专题：围绕测试、调试、性能分析、profile 与问题定位。
- 面试与表达专题：把知识点串成高频问答、项目案例和答题模板。

## 推荐学习路线

### 第 1 阶段：语言与思维方式

- [Go 语言基础与常用法](./go-language-practices.md)
- [Go 并发模型与 Runtime 心智模型](./go-runtime-concurrency.md)
- [Go 锁与读写锁的底层实现解析](./go-mutex-rwmutex-source.md)

这一阶段重点解决的问题：

- Go 和 Java / Python / Node 的设计哲学有什么不同
- `slice`、`map`、`struct`、`interface`、`error` 应该怎么理解
- goroutine、channel、`context`、锁到底该怎么配合
- `Mutex` 与 `RWMutex` 到底是怎么在 runtime 和 semaphore 上跑起来的

### 第 2 阶段：工程化与服务开发

- [Go 项目工程化与后端实践](./go-project-engineering.md)
- [Go Modules 与依赖管理详解](./go-mod-dependency-management.md)
- [Gin 框架实战指南](./go-gin-guide.md)

这一阶段重点解决的问题：

- Go 项目目录怎么组织
- 配置、日志、错误处理、测试、发布怎么做更稳
- `go.mod`、`go.sum`、`go get`、`replace`、`go work` 之间是什么关系
- 基于 `net/http` 和 Gin 写 API 服务时，边界应该怎么划分

### 第 3 阶段：微服务与 RPC

- [go-zero 架构与实战认知](./go-zero-guide.md)
- [gRPC 通信协议深度解读](./go-grpc-deep-dive.md)
- [Go 面经](./面经.md)

这一阶段重点解决的问题：

- go-zero 为什么适合中大型 Go 微服务
- `goctl`、`ServiceContext`、`logic` 层、RPC 层分别承担什么职责
- gRPC 在协议层到底做了什么，为什么它会比“普通 JSON over HTTP”更适合某些场景
- 怎么把已有知识讲成更像真实项目经验的面试答案

## 体系化文档导航

| 文档 | 适合谁 | 核心内容 |
|------|--------|----------|
| [go-language-practices.md](./go-language-practices.md) | Go 初学者、跨语言工程师 | 语法、类型系统、方法、接口、泛型、错误处理、标准库常用法 |
| [go-runtime-concurrency.md](./go-runtime-concurrency.md) | 正在写并发代码的开发者 | GMP 调度、goroutine、channel、`sync`、`atomic`、`context`、内存模型 |
| [go-mutex-rwmutex-source.md](./go-mutex-rwmutex-source.md) | 想把锁机制吃透的开发者 | 基于 Go 1.26.1 源码理解 `Mutex` / `RWMutex` 的状态位、快慢路径、饥饿模式与信号量协作 |
| [go-project-engineering.md](./go-project-engineering.md) | 需要把代码变成“项目”的开发者 | `go mod`、项目布局、配置、日志、可观测性、测试、构建发布 |
| [go-mod-dependency-management.md](./go-mod-dependency-management.md) | 想彻底搞懂模块与依赖管理的开发者 | `go.mod`、`go.sum`、MVS、`go get`、`replace`、私有模块、`go work`、vendor |
| [go-gin-guide.md](./go-gin-guide.md) | 做 HTTP API / BFF / 管理后台的开发者 | 路由、绑定、校验、中间件、错误处理、性能与安全实践 |
| [go-zero-guide.md](./go-zero-guide.md) | 要落地 Go 微服务体系的团队 | go-zero 设计思路、代码生成、REST/RPC 分层、服务治理 |
| [go-grpc-deep-dive.md](./go-grpc-deep-dive.md) | 想把 RPC 与协议层搞明白的开发者 | Protobuf、HTTP/2、stream、metadata、deadline、状态码、拦截器 |
| [面经.md](./面经.md) | 正在准备面试或复盘项目表达的开发者 | 高频问答、深挖题、go-zero/gRPC/GORM 案例、项目回答模板 |

## 全量文档导航

这部分按主题把当前目录下的所有文档都列全，适合做索引页使用。

### A. 体系化主线

| 文档 | 定位 | 关键词 |
|------|------|--------|
| [go-language-practices.md](./go-language-practices.md) | 语言基础主线 | 语法、类型、方法、接口、泛型、错误 |
| [go-runtime-concurrency.md](./go-runtime-concurrency.md) | 并发与 runtime 主线 | GMP、goroutine、channel、`sync`、内存模型 |
| [go-mutex-rwmutex-source.md](./go-mutex-rwmutex-source.md) | 锁源码专题 | `Mutex`、`RWMutex`、state、sema、饥饿模式 |
| [go-project-engineering.md](./go-project-engineering.md) | 项目工程化主线 | 分层、配置、日志、测试、构建、发布 |
| [go-mod-dependency-management.md](./go-mod-dependency-management.md) | 依赖管理主线 | `go.mod`、`go.sum`、`go get`、MVS、私有模块、`go work` |
| [go-gin-guide.md](./go-gin-guide.md) | HTTP 服务专题 | Gin、handler、middleware、request context |
| [go-zero-guide.md](./go-zero-guide.md) | 微服务框架专题 | `goctl`、`ServiceContext`、REST、RPC、治理 |
| [go-grpc-deep-dive.md](./go-grpc-deep-dive.md) | RPC 协议专题 | Protobuf、HTTP/2、metadata、deadline、stream |

### B. 基础心智与高频避坑

| 文档 | 主题 | 关键词 |
|------|------|--------|
| [go-slice-pitfalls.md](./go-slice-pitfalls.md) | 切片 | 底层数组、扩容、共享、截断 |
| [go-map-pitfalls.md](./go-map-pitfalls.md) | map | 零值、并发写、遍历顺序、初始化 |
| [go-pointer-pitfalls.md](./go-pointer-pitfalls.md) | 指针 | 值/引用语义、接收者、逃逸直觉 |
| [go-interface-pitfalls.md](./go-interface-pitfalls.md) | 接口 | 动态类型、`nil interface`、抽象边界 |
| [go-error-pitfalls.md](./go-error-pitfalls.md) | 错误处理 | 包装、比较、错误链、设计方式 |
| [go-context-pitfalls.md](./go-context-pitfalls.md) | 上下文 | 超时、取消、`Value`、链路传递 |
| [go-goroutine-pitfalls.md](./go-goroutine-pitfalls.md) | goroutine | 泄漏、关闭、生命周期、协作退出 |
| [go-channel-pitfalls.md](./go-channel-pitfalls.md) | channel | close、阻塞、select、同步语义 |
| [go-sync-pitfalls.md](./go-sync-pitfalls.md) | 同步原语 | `Mutex`、`RWMutex`、`WaitGroup`、`Once`、`Cond` |

### C. 工程排障与质量保证

| 文档 | 主题 | 关键词 |
|------|------|--------|
| [go-testing-pitfalls.md](./go-testing-pitfalls.md) | 测试 | 单测、表驱动、并发测试、边界条件 |
| [go-debugging-workflow.md](./go-debugging-workflow.md) | 调试 | 复现、缩小范围、日志、`dlv`、堆栈 |
| [go-performance-profiling.md](./go-performance-profiling.md) | 性能分析 | benchmark、`pprof`、trace、mutex/block profile |

### D. 面试与项目表达

| 文档 | 主题 | 关键词 |
|------|------|--------|
| [面经.md](./面经.md) | 面试串讲 | 高频八股、GC、逃逸分析、go-zero、gRPC、GORM、场景题、项目表达 |

## 已有避坑与排障专题

下面这些文档更像“专题修炼”或“线上排障经验”。

### 类型与数据结构

- [Go slice 避坑指南](./go-slice-pitfalls.md)
- [Go map 避坑指南](./go-map-pitfalls.md)
- [Go pointer 避坑指南](./go-pointer-pitfalls.md)
- [Go interface 避坑指南](./go-interface-pitfalls.md)

### 错误、上下文与并发

- [Go error 避坑指南](./go-error-pitfalls.md)
- [Go context 避坑指南](./go-context-pitfalls.md)
- [Go goroutine 避坑指南](./go-goroutine-pitfalls.md)
- [Go channel 避坑指南](./go-channel-pitfalls.md)
- [Go sync 避坑指南](./go-sync-pitfalls.md)

### 工程调试与性能

- [Go 测试避坑指南](./go-testing-pitfalls.md)
- [Go 调试方法](./go-debugging-workflow.md)
- [Go 性能分析与 Profiling 方法](./go-performance-profiling.md)

## 建议的使用方式

### 如果你正在准备面试或系统复习

按这个顺序读：

1. `go-language-practices.md`
2. `go-runtime-concurrency.md`
3. `go-mutex-rwmutex-source.md`
4. `go-project-engineering.md`
5. `go-mod-dependency-management.md`
6. `go-grpc-deep-dive.md`
7. 结合具体方向再读 `go-gin-guide.md` 或 `go-zero-guide.md`
8. 最后用 `面经.md` 做开口训练

### 如果你正在做业务项目

按问题驱动读：

- 理解锁竞争、饥饿模式、读写锁：先看 `go-mutex-rwmutex-source.md`
- 写 API 服务：先看 `go-gin-guide.md`
- 上微服务：先看 `go-zero-guide.md`
- 对接内部 RPC：先看 `go-grpc-deep-dive.md`
- 处理模块、私有仓库、依赖升级：先看 `go-mod-dependency-management.md`
- 排查并发 bug：先看 `go-runtime-concurrency.md`，再回到 `go-goroutine-pitfalls.md`、`go-channel-pitfalls.md`、`go-sync-pitfalls.md`
- 做压测和性能优化：先看 `go-project-engineering.md`，再看 `go-performance-profiling.md`
- 准备 Go 面试或项目复盘表达：直接看 `面经.md`

### 如果你正在搭建团队规范

建议用这几篇做“项目约定基线”：

1. `go-project-engineering.md`
2. `go-mod-dependency-management.md`
3. `go-gin-guide.md` 或 `go-zero-guide.md`
4. `go-grpc-deep-dive.md`
5. `go-testing-pitfalls.md`
6. `go-debugging-workflow.md`

## 一句话总结

这套 Go 文档的目标不是只讲“语法”，而是把下面这些内容串成一套完整认知：

- Go 语言本身怎么用
- Go 为什么这样设计
- Go 的锁、调度、同步原语在底层是怎么工作的
- Go 服务在真实项目里怎么落地
- Go 模块与依赖管理如何长期稳定维护
- Gin、go-zero、gRPC 在系统中的角色分别是什么
- 当系统变慢、报错、阻塞、超时、泄漏时，该从哪里下手
