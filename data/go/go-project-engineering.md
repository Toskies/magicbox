# Go 项目工程化与后端实践

会写 Go 代码，和能把 Go 项目长期稳定地跑在生产环境里，是两件事。

真正决定项目质量的，往往不是某个语法点，而是下面这些工程问题：

- 目录怎么组织
- 模块怎么管理
- 配置和依赖怎么注入
- 日志、错误、指标、链路怎么打通
- 测试、构建、发布怎么标准化

这篇文档聚焦的就是这些“把代码变成项目”的能力。

## 1. 先明确 Go 项目的目标形态

一个可维护的 Go 后端项目，通常应该满足：

- 新同学能快速找到入口
- transport、业务、存储边界清晰
- 配置和外部依赖能被替换和测试
- 日志、监控、追踪能定位问题
- 本地、测试、生产环境行为尽量一致

所以工程化的核心不是目录多漂亮，而是：

- 信息是否可发现
- 边界是否明确
- 变更是否容易验证

## 2. 常见目录结构

Go 项目没有官方唯一标准，但下面这类结构在服务端项目里很常见：

```text
project/
├── cmd/
│   └── api/
│       └── main.go
├── internal/
│   ├── config/
│   ├── handler/
│   ├── service/
│   ├── repository/
│   └── transport/
├── pkg/
├── api/
├── configs/
├── scripts/
├── deploy/
├── go.mod
└── go.sum
```

### `cmd/`

放程序入口。

适合：

- API 服务入口
- worker 入口
- 定时任务入口
- 管理命令入口

### `internal/`

放项目内部实现。Go 会限制外部模块直接 import `internal` 下的包。

这很适合承载：

- 业务实现
- 内部基础设施
- 不希望暴露给外部复用的代码

### `pkg/`

如果你确实有要给别的项目复用的公共包，可以放这里。

但很多项目会滥用 `pkg/`，导致“凡是不知道放哪的都扔进去”。

更稳妥的做法是：

- 先默认放 `internal/`
- 只有明确要复用、边界足够稳定时，再考虑提炼到 `pkg/`

## 3. `go mod`、依赖管理与版本控制

### `go.mod` 是模块边界

Go 模块化的核心文件是 `go.mod`。

常用命令：

```bash
go mod init github.com/example/project
go mod tidy
go list -m all
```

经验规则：

- 提交 `go.mod` 和 `go.sum`
- 依赖变更后及时 `go mod tidy`
- 不要手工乱改一堆无用依赖

如果你想把 `go.mod`、`go.sum`、`go get`、MVS、`replace`、私有模块、`go work` 的关系一次性理顺，建议继续阅读：

- [Go Modules 与依赖管理详解](./go-mod-dependency-management.md)

### 什么时候用 `go work`

如果你在 mono-repo 或多模块协同开发，可以考虑 `go work`。

适合：

- 多个模块需要本地联调
- 框架模块和业务模块一起开发

不适合：

- 单模块普通服务

### 依赖升级要有节奏

建议：

- 定期升级，而不是长期堆积
- 升级前看 release note
- 升级后跑测试、压测、集成验证

工具上可以结合：

- `go list -m -u all`
- `govulncheck ./...`

## 4. 依赖注入与初始化顺序

Go 没有强制 DI 框架，大多数项目会选择“显式装配”。

例如在 `main` 里做：

1. 读取配置
2. 初始化日志
3. 初始化数据库 / Redis / MQ / RPC 客户端
4. 组装 repository
5. 组装 service
6. 组装 HTTP / gRPC server
7. 启动服务

这类显式装配的好处是：

- 依赖一目了然
- 调试更直接
- 测试更容易替换 mock / fake

### 少用全局单例

全局变量会带来：

- 测试隔离困难
- 生命周期管理混乱
- 并发与配置边界不清晰

例如：

- 全局 DB 连接
- 全局 Config 对象
- 全局 Logger

不是绝对不能用，但要慎重。

## 5. 配置管理

一个成熟的 Go 服务通常需要同时面对：

- 本地开发配置
- 测试环境配置
- 生产环境配置
- 容器与云原生环境变量

### 推荐思路

- 静态配置走配置文件或环境变量
- 敏感信息优先走环境变量或密钥系统
- 配置在启动时完成解析和校验

配置对象应该尽早转成强类型结构体：

```go
type Config struct {
	Server struct {
		Addr string `json:"addr"`
	}
	MySQL struct {
		DSN string `json:"dsn"`
	}
}
```

### 配置设计的几个原则

- 不要到处直接读环境变量
- 不要在业务深处偷偷读配置
- 启动时尽可能校验关键配置
- 默认值要清晰且可解释

## 6. 分层设计：别让 handler 变成“巨石函数”

Go 服务端常见分层可以简化成：

- transport 层：HTTP / gRPC / MQ 消费入口
- service / usecase 层：业务编排
- repository / dao 层：数据库、缓存、外部系统访问
- domain 层：核心对象与规则

这不是教条，而是为了回答两个问题：

1. 哪一层负责协议细节？
2. 哪一层负责业务规则？

### 一个实用原则

尽量让 transport 层只关心：

- 参数解析
- 调用业务层
- 返回结果

而不要把真正业务逻辑、事务、复杂流程编排堆在 handler 里。

## 7. 错误处理与错误分层

Go 项目的错误处理，建议至少区分三层：

- 底层错误：数据库、网络、I/O、第三方 SDK
- 业务错误：用户不存在、余额不足、状态非法
- 接口错误：HTTP / gRPC 对外响应格式

### 关键原则

- 底层错误向上包装，不要丢上下文
- 业务错误尽量有稳定可判断的类型或码
- 对外返回要做统一翻译，不要把内部报错直接裸露给客户端

例如：

```go
if err := repo.Create(ctx, user); err != nil {
	return fmt.Errorf("create user in repo: %w", err)
}
```

然后在更上层把它映射成：

- HTTP 400 / 404 / 500
- gRPC `codes.InvalidArgument` / `codes.NotFound` / `codes.Internal`

相关专题见：

- [Go error 避坑指南](./go-error-pitfalls.md)

## 8. 日志、指标、链路追踪

后端服务的可观测性，建议最少包含三件事：

- 日志
- metrics
- tracing

### 日志

推荐结构化日志，至少包含：

- 时间
- 等级
- 请求 ID / trace ID
- 关键业务 ID
- 错误上下文

常见选择：

- 标准库 `log/slog`
- `zap`
- `zerolog`

### Metrics

常见指标：

- QPS
- RT / latency
- error rate
- goroutine 数量
- GC 指标
- 数据库连接池状态

### Tracing

适合排查：

- 某个请求慢在哪一跳
- 哪个下游服务超时
- 一条请求链跨服务如何流转

在 Go 里，tracing 常常和 `context` 一起传递。

## 9. 数据库与外部依赖实践

### 数据库连接池

常见误区：

- 只建一个 DB 对象但不管连接池参数
- 并发升高后连接池耗尽
- 本地没问题，线上全超时

需要关注：

- 最大打开连接数
- 最大空闲连接数
- 连接最大生命周期

### 外部调用必须有超时

包括：

- HTTP client
- gRPC client
- SQL
- Redis
- MQ

如果没有超时控制，故障时整个系统会很容易被拖死。

### 做好幂等、重试与熔断边界

重试不是默认应该开，而是要先确认：

- 操作是否幂等
- 下游错误是否适合重试
- 重试是否会放大雪崩

## 10. 测试策略

高质量 Go 项目通常会把测试分成几层。

### 单元测试

验证：

- 纯业务规则
- 边界条件
- 错误分支

### 集成测试

验证：

- 数据库交互
- HTTP / gRPC 接口联动
- 配置与基础设施装配

### Benchmark

验证：

- 热路径性能
- 分配次数
- 算法和数据结构优化效果

Go 的测试工具链足够强，建议至少熟练掌握：

```bash
go test ./...
go test -race ./...
go test -run TestName -v ./...
go test -bench . -benchmem ./...
```

相关专题见：

- [Go 测试避坑指南](./go-testing-pitfalls.md)
- [Go 调试方法](./go-debugging-workflow.md)
- [Go 性能分析与 Profiling 方法](./go-performance-profiling.md)

## 11. 构建、发布与部署

### 常见构建命令

```bash
go build ./...
go build -o bin/app ./cmd/api
```

交叉编译示例：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/app ./cmd/api
```

### 版本信息注入

常见做法是用 `-ldflags` 注入版本号、提交号、构建时间。

例如：

```bash
go build -ldflags "-X main.version=1.0.0 -X main.commit=$(git rev-parse --short HEAD)"
```

### 容器化建议

- 二进制尽量小而清晰
- 区分构建镜像和运行镜像
- 健康检查与优雅退出要可观测

## 12. 优雅退出与服务生命周期

一个成熟的服务必须能回答：

- 收到终止信号后，如何停止接收新请求
- 在处理中请求如何收尾
- 连接池、worker、后台任务如何关闭

Go 服务常见做法：

- 监听系统信号
- 调用 `Shutdown`
- 为关闭过程设置超时

这类能力在 HTTP、gRPC、worker 中都非常关键。

## 13. 常用工具链

建议熟悉这些工具：

- `gofmt`
- `goimports`
- `go vet`
- `golangci-lint`
- `govulncheck`
- `dlv`
- `pprof`

经验规则：

- 格式化交给工具，不靠人工
- lint 是兜底，不是设计替代品
- profile 数据比“感觉优化”更重要

## 14. 一个适合 Go 后端服务的最小清单

如果你在开一个新 Go 服务，最少建议有：

- `go.mod`
- 清晰的入口目录
- 强类型配置结构
- 日志初始化
- 超时与取消控制
- 错误包装与统一返回
- 基本单元测试
- `-race` 检查
- 健康检查与优雅退出

## 15. 一句话总结

Go 工程化的本质，是把“代码能跑”升级成：

- 项目结构清晰
- 依赖关系可控
- 线上问题能看见
- 变更可以验证
- 团队协作成本可接受
