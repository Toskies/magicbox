# Go Modules 与依赖管理详解

很多人会用 Go，但对模块与依赖管理的理解只停留在：

- 有个 `go.mod`
- 有个 `go.sum`
- 缺包了就 `go get`
- 最后再 `go mod tidy`

这套用法在简单项目里通常能跑，但一旦项目变复杂，就会开始碰到下面这些问题：

- `go get` 和 `go install` 到底谁该用
- `replace` 应该什么时候加，什么时候删
- 为什么 `go mod tidy` 会改一堆内容
- 为什么别人机器能拉到包，我这里不行
- 私有仓库为什么会和 proxy / sumdb 冲突
- mono-repo 下到底该用多模块还是 `go work`

这篇文档就是把这些关系系统讲清楚。

## 0. 版本说明

以下内容基于当前本机工具链行为整理：

- `go version go1.26.1 linux/amd64`

同时参考了本机 `go help` 输出，包括：

- `go help mod`
- `go help modules`
- `go help get`
- `go help mod edit`
- `go help work`

## 1. 先分清 3 个概念：package、module、workspace

很多混乱都来自这 3 个词没分开。

### package

Go 代码组织和编译的基本单位。

通常：

- 一个目录对应一个 package
- 通过 `package xxx` 声明
- 通过 import 路径被别的包引用

### module

Go 依赖管理与版本发布的基本单位。

它由模块根目录下的 `go.mod` 定义。

可以把 module 理解成：

- 一组一起发布、一起打版本的 package 集合

### workspace

多个 module 的本地协作工作区，由 `go.work` 定义。

适合：

- 多模块联调
- mono-repo 局部解耦
- 本地同时开发多个相互依赖模块

一个最常见的误解是：

- 以为 package、module、repo 这三个概念必须完全重合

实际上它们经常重合，但不必总是重合。

## 2. `go.mod` 到底在表达什么

`go.mod` 的核心作用不是“记一下依赖列表”，而是表达：

- 当前模块是谁
- 当前模块期望的 Go 版本 / 工具链
- 直接依赖和若干解析规则

最常见结构：

```go
module github.com/example/project

go 1.26

require (
	github.com/gin-gonic/gin v1.10.1
	google.golang.org/grpc v1.76.0
)
```

### 最常用的几类条目

- `module`
- `go`
- `toolchain`
- `require`
- `replace`
- `exclude`

### 当前 Go 版本里更高级的条目

从 `go help mod edit` 可以看到，当前版本还支持：

- `retract`
- `tool`
- `ignore`
- `godebug`

但对大多数业务项目来说，前 6 类最关键。

## 3. `module`、`go`、`toolchain` 三行分别是什么

### `module`

定义模块路径。

例如：

```go
module github.com/acme/payment-service
```

这个路径会影响：

- 其他模块如何 import 你
- 模块版本如何在代理和仓库中定位

### `go`

声明该模块的 Go 语言 / 模块语义版本基线。

例如：

```go
go 1.26
```

它不是简单的“注释”，而会影响工具链对某些行为的处理。

### `toolchain`

用于建议或固定使用某个 Go 工具链版本。

当前 Go 帮助里也给出：

```bash
go get toolchain@patch
```

这说明现在工具链本身也被纳入模块化管理的一部分。

## 4. `require`：依赖声明不是“全量树”，而是主模块的需要

`require` 记录的是：

- 当前模块构建所需的其他模块版本

要注意：

- 这不是把整棵依赖树所有节点都平铺展开
- 更不是“越多写得越全越好”

Go 会结合依赖图和版本选择算法，解析出真正用于构建的模块集合。

### `// indirect` 是什么意思

你会经常看到：

```go
require github.com/some/mod v1.2.3 // indirect
```

它表示：

- 当前模块没有显式 import 这个模块里的包
- 但构建依赖图需要它

不要看到 `indirect` 就手痒想删。是否应该删，交给 `go mod tidy` 决定更稳。

## 5. `go.sum`：它不是备份清单，而是完整性校验记录

很多人会误解 `go.sum`：

- 以为它只是缓存
- 或者以为删了也没关系

更准确地说，`go.sum` 是：

- 对模块内容与 `go.mod` 文件的校验和记录

它帮助工具链确认：

- 你拿到的依赖内容是不是和之前记录的一致

### 实践规则

- `go.sum` 应该提交到版本库
- 不要因为“看着乱”就手工删
- 真要清理，也应该通过 `go mod tidy`、重新拉依赖等标准动作完成

## 6. Go 是怎么选依赖版本的：MVS 心智模型

Go 模块解析的一个关键机制是 MVS，通常指 Minimal Version Selection。

你可以先建立一个非常实用的直觉：

- Go 倾向于选择“满足约束所需的最小版本集合”
- 而不是像有些生态那样做复杂的 SAT 求解或多版本并存

这会带来几个结果：

- 构建结果更可预测
- 依赖图更稳定
- 但你也要理解为什么某个升级可能带动整条链变化

### 一个很重要的现实感受

当你执行：

```bash
go get example.com/pkg@v1.2.3
```

你以为自己只改了一个包，但实际工具链可能为了满足整体图约束，顺带调整其他模块版本。

所以：

- 不要把依赖升级理解成“只改一行 require”
- 要把它看成一次依赖图求值

## 7. 日常最常用的命令到底该怎么分工

### `go mod init`

初始化新模块：

```bash
go mod init github.com/example/project
```

### `go get`

日常增加、升级、降级、删除依赖，主力命令是它。

当前 `go help get` 明确写了：

- 日常添加、移除、升级依赖应使用 `go get`

常见用法：

```bash
go get github.com/gin-gonic/gin
go get google.golang.org/grpc@v1.76.0
go get example.com/mod@none
go get go@latest
go get toolchain@patch
```

你可以把它理解成：

- 修改 `go.mod`
- 解析依赖图
- 下载所需源码到模块缓存

### `go mod tidy`

用于：

- 补齐缺失依赖
- 移除不再需要的依赖
- 清理 `go.mod` / `go.sum`

它不是：

- “升级所有依赖”
- “无脑整理一下格式”

### `go mod download`

用于提前下载模块到本地缓存。

适合：

- CI 预热缓存
- 想提前拉齐依赖
- 容器构建优化

### `go mod graph`

查看模块依赖图。

适合：

- 排查版本链
- 理解为什么某个模块被引入

### `go mod why`

回答：

- 这个包 / 模块为什么会出现在依赖里

### `go mod verify`

校验缓存中依赖内容与记录是否一致。

### `go mod vendor`

把依赖复制到 `vendor/` 目录。

只在团队或构建环境明确需要 vendor 策略时再用，不是所有项目都必须开。

## 8. `go get` 和 `go install` 到底怎么分

这是很多 Go 开发者最容易混淆的地方。

### `go get`

职责是：

- 调整当前模块的依赖

它关注的是：

- 当前项目的 `go.mod`

### `go install`

职责是：

- 构建并安装某个命令行工具

尤其是带版本时：

```bash
go install example.com/cmd/tool@latest
```

当前帮助明确说明：

- 指定版本的 `go install` 会忽略当前目录的 `go.mod`

所以：

- 给项目加业务依赖，用 `go get`
- 装开发工具，用 `go install pkg@version`

不要混着用。

## 9. 几个最常见的依赖管理场景

### 场景一：新增业务依赖

```bash
go get github.com/gin-gonic/gin
go mod tidy
```

### 场景二：升级到指定版本

```bash
go get google.golang.org/grpc@v1.76.0
go mod tidy
```

### 场景三：删除一个模块依赖

```bash
go get example.com/legacy/mod@none
go mod tidy
```

### 场景四：看哪些模块可以升级

```bash
go list -m -u all
```

### 场景五：查看完整模块图

```bash
go mod graph
```

## 10. `replace`：本地联调很好用，但要有边界

`replace` 是非常实用也非常容易被滥用的功能。

例如：

```go
replace github.com/acme/common => ../common
```

或者：

```go
replace github.com/acme/common v1.2.3 => github.com/acme/common v1.2.4-fix
```

### 适合用 `replace` 的场景

- 本地联调多个模块
- 临时替换一个修复分支
- 在模块尚未正式发布前做短期验证

### 不适合长期滥用的场景

- 团队长期依赖本地路径替换
- 生产构建隐式依赖开发机目录
- 用 `replace` 掩盖版本管理混乱

### 一个实用原则

- 本地开发可以短期 `replace`
- 正式协作最好尽快发版，回到正常版本依赖

## 11. `exclude` 和 `retract` 是什么

### `exclude`

用于在当前模块中排除某个版本。

适合：

- 明确知道某个版本不能用

### `retract`

更偏向模块发布者视角，用于声明：

- 某个已发布版本应被撤回或不建议使用

日常业务项目最常改的是：

- `require`
- `replace`

`exclude` 和 `retract` 频率相对低，但你至少要认识它们。

## 12. 私有模块：为什么总和代理、校验、认证扯在一起

`go help modules` 提到了一些关键环境变量：

- `GOPROXY`
- `GOSUMDB`
- `GOPRIVATE`
- `GONOPROXY`
- `GONOSUMDB`

这几个变量经常一起出现。

### 为什么要配 `GOPRIVATE`

因为默认情况下，Go 可能会：

- 去代理拉模块
- 去 checksum database 校验

对于私有模块，这通常不是你想要的行为。

### 一个常见配置思路

例如公司私有域名是 `git.corp.example.com`，常见思路会是：

```bash
go env -w GOPRIVATE=git.corp.example.com
```

必要时再进一步控制：

- `GONOPROXY`
- `GONOSUMDB`

### `GOPROXY` 不一定全世界都一样

当前机器上的 `GOPROXY` 就可能是：

```text
https://goproxy.cn,direct
```

而别人的机器可能是：

```text
https://proxy.golang.org,direct
```

所以排查依赖拉取问题时，第一步之一就是：

```bash
go env GOPROXY GOSUMDB GOPRIVATE GONOPROXY GONOSUMDB
```

## 13. 模块缓存与 vendor

### 模块缓存

Go 会把下载过的模块放到模块缓存里。

当前机器上的模块缓存位置可以通过：

```bash
go env GOMODCACHE
```

查看。

### 什么时候关心缓存

适合：

- CI 做缓存
- 排查“为什么老是重复下载”
- 需要清理损坏或异常缓存

### `vendor` 什么时候值得用

只有在这些场景下才更值得考虑：

- 内网或离线构建要求强
- 组织规范要求 vendor 入库
- 构建环境必须完全避免动态拉包

如果团队没有这类约束，很多普通服务项目不一定需要 `vendor/`。

## 14. `go work`：多模块联调的正规姿势

如果你有多个模块需要本地联调，`go work` 往往比一堆临时 `replace` 更整洁。

示例：

```text
go 1.18

use (
  ../foo/bar
  ./baz
)
```

从 `go help work` 可以看出：

- `go.work` 通过 `use` 指定一组本地模块目录
- 这些模块会作为 workspace 的主模块集合参与构建

### 常用命令

```bash
go work init
go work use ./moduleA ../moduleB
go work sync
```

### 什么时候用 `go work`

适合：

- 多模块一起开发
- 框架模块和业务模块联调
- mono-repo 下做明确边界的多模块协作

不太适合：

- 单模块普通项目
- 团队并没有真正多模块协作需求

### `go.work` 要不要提交

这没有一刀切答案。

经验规则：

- 如果它是团队正式工作区约定，可以提交
- 如果它只是你本地临时联调工具，通常不建议提交

## 15. 当前版本 `go.mod` 的一些高级能力

从 `go help mod edit` 可以看到，现在 `go.mod` 还能表达：

- `tool`
- `ignore`
- `godebug`

### `tool`

适合把项目工具依赖纳入模块管理，而不是全靠 README 口头约定。

### `ignore`

适用于一些更高级的模块行为控制场景，普通业务项目较少直接手改。

### `godebug`

用于声明某些运行时 / 兼容行为的调试开关。

这几项说明一个趋势：

- Go 的模块系统不再只是“依赖版本表”
- 它正逐渐承载更多工程约定

## 16. 一个推荐的日常工作流

### 新项目

```bash
go mod init github.com/acme/project
go mod tidy
```

### 增加依赖

```bash
go get example.com/pkg
go mod tidy
```

### 升级依赖

```bash
go get example.com/pkg@v1.2.3
go mod tidy
go test ./...
```

### 排查依赖链

```bash
go mod why example.com/pkg
go mod graph
go list -m -u all
```

### 做本地多模块联调

优先考虑：

- `go work`

其次才是：

- 临时 `replace`

## 17. 最常见的误区

### 误区一：`go mod tidy` 等于升级依赖

不是。

它主要是清理和补齐，不是做全面升级。

### 误区二：`go install` 可以给当前项目加依赖

不是。

它更适合装工具，不是管理当前模块业务依赖。

### 误区三：`replace` 可以长期不清理

这是很多项目后期依赖混乱的源头。

### 误区四：`go.sum` 不重要，可以不提交

这会让团队协作和依赖校验都变得更脆弱。

### 误区五：私有模块拉不下来时，只盯着仓库权限

很多时候真正的问题在：

- `GOPRIVATE`
- `GOPROXY`
- `GOSUMDB`
- SSH / HTTPS 凭据

## 18. 建议搭配阅读

如果你正在把 Go 项目落地成团队工程，建议顺着读：

1. [Go 项目工程化与后端实践](./go-project-engineering.md)
2. 本文
3. [Gin 框架实战指南](./go-gin-guide.md) 或 [go-zero 架构与实战认知](./go-zero-guide.md)
4. [gRPC 通信协议深度解读](./go-grpc-deep-dive.md)

这样你会把：

- 模块边界
- 依赖治理
- 服务实现
- 服务间通信

这几层串起来。

## 19. 一句话总结

Go 的依赖管理不是“会几个命令”就够了。

真正重要的是理解：

- package、module、workspace 的边界
- `go.mod` 与 `go.sum` 各自负责什么
- `go get`、`go install`、`go mod tidy` 如何分工
- `replace`、私有模块、`go work` 在团队协作里该怎么使用

只有把这些关系理顺，项目依赖才会长期稳定，而不是越维护越乱。
