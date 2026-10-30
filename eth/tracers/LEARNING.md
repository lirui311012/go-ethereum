# eth/tracers：执行跟踪

[返回 eth](../LEARNING.md) · [返回学习总览](../../LEARNING.md) · 相关 [core/vm](../../core/vm/LEARNING.md)、[internal/ethapi](../../internal/ethapi/LEARNING.md)

`eth/tracers` 在 EVM 执行时记录发生了什么：调用树、每步操作码、状态访问。当前的挂接点是 `core/tracing.Hooks`。`vm.Config.Tracer` 的类型就是 `*tracing.Hooks`。调试 API 在本包的 `API`，由 `debug` 命名空间暴露。

## 建议阅读顺序

1. `core/tracing/hooks.go`：`Hooks` 上有哪些回调（`OnTxStart`、`OnEnter`、`OnOpcode`、`OnExit`）。
2. `eth/tracers/dir.go`：`DefaultDirectory.Register`、`Tracer`。
3. `eth/tracers/native/call.go`：`callTracer` 如何填 `Hooks`。
4. `eth/tracers/logger/logger.go`：`StructLogger`，逐步结构日志。
5. `eth/tracers/api.go`：`TraceTransaction`、`TraceCall`、`TraceBlockByNumber`、`traceTx`。
6. `eth/tracers/js/goja.go`：用 JavaScript 写的 tracer。
7. `eth/tracers/live.go`：`LiveDirectory`，区块导入时附带的 tracer。

## 两种用法

**按请求跟踪。** `API.TraceTransaction` 找到交易所在的块，在父状态上重放前面的交易，再对目标交易调用 `traceTx`。`TraceCall` 用 `ethapi.TransactionArgs` 在指定块上做一次 `eth_call` 式的执行，同样把 `Hooks` 放进 `vm.Config`。`TraceBlockByNumber`、`TraceBlockByHash`、`TraceBlock` 对块里每一笔都跟踪。`TraceChain` 跨多个块，结果用订阅推送。

`traceTx` 根据配置从 `DefaultDirectory` 取出 tracer。没有指定名字时，结构化日志走 `logger`。超时默认 `defaultTraceTimeout`（5 秒），超时后执行被中止。

**导入时跟踪。** `LiveDirectory` 注册的 tracer 在正常插块时运行，用来做供应统计这类长期观察。`eth/tracers/live/supply.go` 是一个例子。它和 `debug_traceTransaction` 不是同一条入口。

## 自带的 tracer

`native` 在 `init` 里注册到 `DefaultDirectory` 的名字有 `callTracer`、`flatCallTracer`、`prestateTracer`、`4byteTracer`、`erc7562Tracer`、`muxTracer`、`noopTracer`、`keccak256PreimageTracer`。`opcode_counter.go` 的 `NewOpcodeCounter` 只在代码里构造，没有注册成 RPC 名字。

`muxTracer` 把多个 tracer 的回调并到同一次执行里，避免为一份结果把交易重放多遍。

`logger.NewStructLogger` 记录每步栈、内存和存储变化。`NewAccessListTracer` 在一次执行里收集访问过的地址和槽，给交易补 access list。

JS tracer 用 goja 解释脚本，脚本里的钩子再映射到同一套 `Hooks`。内置脚本在 `eth/tracers/js/internal/tracers`。

## 钩子在哪一刻响

`Hooks` 的字段都是可选的。执行点在 [core/vm](../../core/vm/LEARNING.md) 和 `core` 的区块处理里，不在本包。本包的 tracer 只是把这些函数填上。

同一类事件有 V1 和 V2。运行时先看 V2：`OnGasChangeV2`、`OnEnterV2`、`OnExitV2`、`OnOpcodeV2`。V2 没设才回落到 V1。V1 的 gas 参数是 execution 那一维。只实现 `OnGasChange` 的 tracer，在 EIP-8037/8038 打开后看不到 state gas 的变化。

系统调用走 `OnSystemCallStart` / `OnSystemCallEnd`，中间有帧和状态钩子，没有 `OnTxStart` / `OnTxEnd`。状态变化另有 `OnBalanceChange`、`OnNonceChange`、`OnCodeChange`、`OnStorageChange`、`OnLog`。新 opcode 如果改了余额、存储或 gas 向量，要确认对应钩子在那条路径上仍然被调用。

## 读的时候记住

- 跟踪是重放。`TraceTransaction` 从父状态再执行一遍。
- 回调接口是 `tracing.Hooks`。新 tracer 填这个结构，并在 `DefaultDirectory` 注册名字。
- 打开 HTTP 时默认就有 `eth`，没有 `debug`。远程要调 `debug_*`，需要在 `--http.api` 里加上 `debug`。
- 逐步日志很占内存。整段链用 `TraceChain`，单笔用 `TraceTransaction`。
