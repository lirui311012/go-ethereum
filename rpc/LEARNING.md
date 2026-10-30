# rpc：JSON-RPC 服务

[返回学习总览](../LEARNING.md) · 上一篇 [eth](../eth/LEARNING.md) · 下一步 [internal/ethapi](../internal/ethapi/LEARNING.md)

`rpc` 做 JSON-RPC 2.0 和服务反射注册。以太坊方法在 [internal/ethapi](../internal/ethapi/LEARNING.md)、`eth/filters`、`eth/tracers`、`eth/catalyst`。端点生命周期在 [node](../node/LEARNING.md)。包说明在 `rpc/doc.go`。

## 建议阅读顺序

1. `rpc/doc.go`：方法签名和订阅约定。
2. `rpc/server.go`：`Server`、`RegisterName`、`serveSingleRequest`。
3. `rpc/service.go`：反射注册，`formatName` 把导出方法首字母改成小写。
4. `rpc/handler.go`：单请求、批量、订阅。
5. `rpc/http.go`、`websocket.go`、`endpoints.go`。
6. `node/rpcstack.go`：module 白名单。

## 一次 HTTP 调用

1. `httpServer.ServeHTTP` 按路径把请求交给 handler。
2. `Server.ServeHTTP` 校验方法和 Content-Type。
3. `serveSingleRequest`。HTTP 把 `allowSubscribe` 设为 false。
4. 单条走 `handleMsg`，数组走 `handleBatch`。
5. `serviceRegistry.callback` 用 `_` 切开 namespace 和方法名。
6. `callback.call` 用反射调用业务方法。

批量在同一个 call goroutine 里顺序执行。空批和条数超限会整批返回一个错误。响应字节超限时，已经算完的调用仍返回原结果，还没执行的调用才报响应过大。公开口默认 1000 条、响应 25000000 字节。engine 认证口是 2000 条、响应 250000000 字节。上限为 0 表示不限制。这两个数字写在 `node/defaults.go`，不是 1024 进位的 MiB。

## 传输差异

| 传输 | 行为 |
|------|------|
| HTTP | 一请求一响应。不支持订阅 |
| WebSocket | 长连接，允许 `eth_subscribe` |
| IPC | 注册全部已传入 API，没有 HTTP 那种 module 白名单 |
| 进程内 | `Node.startInProc` 直接 `RegisterName` |

订阅方法必须导出，第一个参数是 `context.Context`，返回 `(*rpc.Subscription, error)`。调用名是 `<namespace>_subscribe`。例如 `eth_subscribe` 加 `"newHeads"` 落到过滤器的 `NewHeads`。

## 加一个方法

落在现有 namespace 时，在对应 service 上加导出方法。签名按 `rpc/doc.go`：返回 0 到 2 个值，error 在最后；可选参数用指针。方法名是首字母小写后接在 namespace 后面。

新 namespace：构造 `rpc.API{Namespace, Service}`，从 `eth.APIs` 返回，或在启动时 `stack.RegisterAPIs`。公开 HTTP/WS 还要把 namespace 放进 `HTTPModules` / `WSModules`（或 `--http.api`）。注册必须发生在 `Node` 仍处于 initializing、`startRPC` 之前。

需要新的链能力时，改 `ethapi.Backend` 和 `eth.EthAPIBackend`，不要让 `internal/ethapi` 直接引用 `*eth.Ethereum`。

## 读的时候记住

- 在结构体上加了方法，只说明它进了注册表。HTTP 还要过 module 白名单。`debug`、`admin`、`txpool` 默认不在 `--http.api` 里。
- `debug` 名下有多套 service，方法合并进同一张表。
- `ethclient` 只包一层 `rpc.Client`，不参与节点内部组装。
