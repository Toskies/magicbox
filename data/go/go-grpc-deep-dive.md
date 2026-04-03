# gRPC 通信协议深度解读

很多人会用 gRPC，但只停留在“写 `.proto`、生成代码、调方法”这个层面。

真正想把 gRPC 用稳、用明白，至少要理解下面几层：

- 契约层：Protobuf 在描述什么
- 传输层：HTTP/2 在承担什么
- 消息层：gRPC 如何组织二进制消息
- 控制层：metadata、deadline、status、stream 如何工作
- 工程层：拦截器、发现、负载、重试、观测如何配合

这篇文档按这个顺序展开。

## 1. gRPC 到底是什么

gRPC 可以理解为一套现代 RPC 框架，核心特点是：

- 使用 Protobuf 描述服务契约
- 默认运行在 HTTP/2 之上
- 使用二进制消息格式
- 原生支持双向流
- 把超时、状态码、metadata、拦截器等能力纳入统一模型

它和“自己封一个 HTTP JSON 接口”的差别，不只是编码格式不同，而是整套调用语义都更统一。

## 2. 从一个最小例子开始

最小 `.proto` 示例：

```proto
syntax = "proto3";

package user.v1;

option go_package = "example.com/project/pb/userv1;userv1";

service UserService {
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
}

message GetUserRequest {
  int64 id = 1;
}

message GetUserResponse {
  int64 id = 1;
  string name = 2;
}
```

代码生成常见形式：

```bash
protoc --go_out=. --go-grpc_out=. api/user.proto
```

然后你会得到：

- message 结构定义
- client stub
- server interface

这就是“契约先行”的核心。

## 3. Protobuf 层：为什么它比普通 JSON 契约更强

### 字段编号是协议的一部分

```proto
message GetUserRequest {
  int64 id = 1;
}
```

这里的 `1` 非常重要，它不是装饰品，而是编码协议里的字段编号。

这意味着：

- 字段改名通常不影响兼容性
- 字段编号一旦对外发布，就要谨慎维护
- 删除字段后最好用 `reserved` 保留编号，防止误复用

### Protobuf 的优势

- 编码更紧凑
- schema 更明确
- 跨语言生成更成熟
- 字段演进策略更清晰

### 设计 `.proto` 时的几个原则

- 字段名语义清晰
- 编号稳定
- 不要随意复用废弃字段号
- 公共 message 尽量复用，但不要把一切都揉成超级公共结构

## 4. HTTP/2 层：gRPC 传输为什么更高效

gRPC 默认运行在 HTTP/2 上。HTTP/2 相比 HTTP/1.1 的关键能力包括：

- 多路复用
- 头部压缩
- 流式传输
- 同一连接上承载多个 stream

这意味着：

- 不需要像 HTTP/1.1 那样频繁建立很多连接才能并发
- 一个 TCP 连接上可以并发跑多个 RPC stream
- streaming 能力更自然

### 一个 RPC 对应一个 HTTP/2 stream

这点非常关键。

一个 gRPC 调用，不是简单的“一次 POST 请求”这么粗糙，而是：

- 建立在 HTTP/2 stream 语义上
- request / response headers、data frames、trailers 都有明确角色

## 5. gRPC 在协议层到底长什么样

### 路径

gRPC 请求路径通常形如：

```text
/package.Service/Method
```

例如：

```text
/user.v1.UserService/GetUser
```

### 常见请求头

通常会看到类似：

```text
:method: POST
:scheme: http
:path: /user.v1.UserService/GetUser
content-type: application/grpc
te: trailers
```

其中：

- `content-type: application/grpc` 表明是 gRPC 内容
- `te: trailers` 表示客户端可以接受 HTTP trailers

### 消息帧格式

每条 gRPC message 在 data frame 里的典型前缀格式是：

- 1 字节压缩标志
- 4 字节大端长度
- N 字节消息体

也就是说，gRPC 并不是“直接把 protobuf 裸扔到 TCP 上”，而是有自己的 message framing。

这也是它能在一个 stream 里发送多条 message 的基础。

## 6. Unary 与 Streaming 的差别

### Unary

最常见，和普通“请求-响应”最像：

- 客户端发 1 条消息
- 服务端回 1 条消息

### Server Streaming

- 客户端发 1 条请求
- 服务端持续回多条消息

适合：

- 分批推送结果
- 日志流
- 大结果集渐进返回

### Client Streaming

- 客户端发多条消息
- 服务端最后回 1 条结果

适合：

- 批量上传
- 连续数据上报

### Bidirectional Streaming

- 客户端和服务端都可以持续发送
- 双方基于同一条 stream 双向通信

适合：

- 实时交互
- 长连接控制流
- 网关与后端流式桥接

真正让 gRPC 和传统 REST 拉开差距的，往往就是 streaming 语义。

## 7. Deadline、取消与 `context`

在 Go 里使用 gRPC 时，`context.Context` 是非常核心的一环。

它通常承载：

- deadline
- cancel
- metadata
- trace 信息

示例：

```go
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()

resp, err := client.GetUser(ctx, &pb.GetUserRequest{Id: 1})
```

### 为什么 deadline 很重要

如果没有 deadline：

- 下游慢了，上游可能一直等
- 故障时 goroutine、连接、资源都会堆积
- 整条调用链更容易雪崩

一个成熟的 gRPC 系统，deadline 不是可选项，而是默认应考虑的能力。

## 8. Metadata：RPC 世界里的“请求头”

metadata 可以理解为 gRPC 中与请求关联的键值对信息。

常见用途：

- token
- request id
- trace id
- 租户信息
- 语言或区域信息

Go 中常见写法：

```go
md := metadata.Pairs("request-id", "abc-123")
ctx := metadata.NewOutgoingContext(context.Background(), md)
```

服务端读取：

```go
md, ok := metadata.FromIncomingContext(ctx)
```

### 使用原则

- metadata 适合传跨边界的控制信息
- 业务主数据仍然应该放在 message body
- 不要把所有参数都塞进 metadata

## 9. gRPC 状态码与错误模型

gRPC 不是只靠 HTTP 状态码表达结果，它有自己的一套状态模型。

常见状态码包括：

- `codes.OK`
- `codes.InvalidArgument`
- `codes.NotFound`
- `codes.AlreadyExists`
- `codes.PermissionDenied`
- `codes.Unauthenticated`
- `codes.DeadlineExceeded`
- `codes.Unavailable`
- `codes.Internal`

服务端示例：

```go
return nil, status.Error(codes.NotFound, "user not found")
```

客户端示例：

```go
st, ok := status.FromError(err)
if ok && st.Code() == codes.NotFound {
	// handle not found
}
```

### 一个很重要的协议细节

gRPC 的最终状态通常通过 HTTP trailers 传递，例如：

- `grpc-status`
- `grpc-message`

这也是为什么 `te: trailers` 会出现在请求头里。

## 10. 拦截器：gRPC 的横切扩展点

拦截器很像 HTTP 中间件，但语义更贴近 RPC。

常见用途：

- 日志
- tracing
- metrics
- 鉴权
- 重试包装
- 统一错误处理

### Unary Interceptor

适合普通一问一答 RPC。

### Stream Interceptor

适合流式 RPC，需要处理收发流的生命周期。

设计上要注意：

- 日志和 trace 往往适合放拦截器
- 复杂业务判断不要硬塞进拦截器

## 11. 连接、发现、负载与重试

真实生产环境里，gRPC 不只是“点对点调一个方法”。

你通常还会遇到：

- 服务发现
- 客户端负载均衡
- 连接复用
- keepalive
- 重试

### 服务发现

常见方式：

- DNS
- 注册中心
- Service Mesh / xDS 体系

### 负载均衡

客户端可以基于发现结果做负载策略。

### 重试要谨慎

不是所有错误都适合重试，必须先确认：

- 调用是否幂等
- 错误是否短暂可恢复
- 重试是否会放大流量

### 长连接也不是“永不出问题”

HTTP/2 长连接带来性能优势，但也需要考虑：

- 连接断开重连
- keepalive 配置
- 代理层兼容性
- 空闲连接与高并发连接模型

## 12. gRPC 与网关、REST、浏览器的关系

gRPC 非常适合服务间通信，但对浏览器原生支持并不像普通 HTTP JSON 那么直接。

因此真实项目里常见几种组合：

- 前端 <-> REST / BFF <-> gRPC 内部服务
- 前端 <-> gRPC-Web 网关 <-> gRPC 服务
- 内部纯服务间全走 gRPC

这意味着 gRPC 不是一定要取代 REST，而是经常和 REST 共存。

## 13. gRPC 设计 `.proto` 时的实战建议

### 建议一：契约优先，兼容优先

- 先想清楚字段语义
- 再分配字段号
- 发布后尽量兼容演进

### 建议二：错误语义明确

不要把所有失败都返回 `Internal`。

更好的做法是让调用方能区分：

- 参数错误
- 未找到
- 权限问题
- 超时
- 下游不可用

### 建议三：不要把 HTTP 语义硬套进所有 RPC

RPC 的边界应该是“能力调用”，不是机械照搬 `/getUser`、`/createUser` 这种 REST 风格命名而不思考服务职责。

### 建议四：考虑消息大小和流式传输

如果返回体很大，不要默认一次性塞成一个超大 message，streaming 往往更合适。

## 14. gRPC 排障时最该先看的东西

当 gRPC 出问题时，优先看：

- deadline 是否太短或未设置
- status code 是什么
- trailers 里返回了什么
- 连接是否建立成功
- 代理或网关是否支持 HTTP/2
- 是否有版本不兼容的 proto 演进

常见问题包括：

- `DeadlineExceeded`
- `Unavailable`
- message too large
- metadata 丢失
- stream 提前关闭

## 15. gRPC 与 Go 微服务框架的关系

像 go-zero 这类框架，往往不是替代 gRPC，而是在 gRPC 之上提供更完整的工程组织方式。

也就是说：

- gRPC 解决的是通信协议与调用模型
- go-zero 解决的是服务工程化与治理协作

把这两层分清楚，很多架构讨论就不会混乱。

## 16. 一句话总结

gRPC 真正的价值不只是“更快的 RPC”，而是：

- 用 Protobuf 明确契约
- 用 HTTP/2 承载多路复用与流式通信
- 用 metadata、deadline、status、interceptor 形成统一调用模型
- 用清晰的协议语义支撑中大型服务系统的长期演进
