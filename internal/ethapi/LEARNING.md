# internal/ethapi：eth 命名空间

[返回学习总览](../../LEARNING.md) · 上一篇 [rpc](../../rpc/LEARNING.md) · 下一步 [accounts](../../accounts/LEARNING.md)

`internal/ethapi` 实现大部分 `eth_*` 和 `txpool_*`。它通过 `Backend` 接口回到全节点，不直接依赖 `*eth.Ethereum`。全节点实现是 `eth.EthAPIBackend`。

## 建议阅读顺序

1. `internal/ethapi/backend.go`：`Backend`、`GetAPIs`。
2. `internal/ethapi/api.go`：`BlockChainAPI.Call`、`TransactionAPI.SendRawTransaction`、`SubmitTransaction`。
3. `eth/api_backend.go`：`SendTx`、`StateAndHeaderByNumberOrHash`、`GetEVM`。
4. `eth/backend.go` 的 `APIs`：miner、admin、net 从这里挂上。
5. `eth/filters`：日志和 `newHeads`。
6. `graphql/service.go`：`--graphql` 时复用同一个 `Backend`，不走 JSON-RPC 注册表。

## eth_sendRawTransaction

1. [rpc](../../rpc/LEARNING.md) 反射到 `TransactionAPI.SendRawTransaction`。
2. `Transaction.UnmarshalBinary`。
3. `SubmitTransaction`：检查 fee cap；未开 unprotected 时要求 `tx.Protected()`。
4. `Backend.SendTx` → `EthAPIBackend.SendTx` → `TxPool.Add`。
5. 临时拒绝且配置了本地跟踪时，跟踪后对调用方返回成功。

这条路径不签名。节点只收已经签好的原始交易。`ethclient.SendTransaction` 也是先 `MarshalBinary`，再调这个方法。

## eth_call

1. `BlockChainAPI.Call` → `DoCall`。
2. `StateAndHeaderByNumberOrHash` → `BlockChain.StateAt`，失败再试历史状态。
3. `TransactionArgs.CallDefaults`、`ToMessage`。
4. `GetEVM` → `vm.NewEVM`。
5. `core.ApplyMessage`。不写状态，不进交易池。超时由 `RPCEVMTimeout` 取消 EVM。

`eth_signTransaction` 走 `TransactionAPI.sign`（`AccountManager.Find` 再 `Wallet.SignTx`）。`eth_sendTransaction` 不调用 `sign`，自己查找钱包并签名，并在签名前拒绝 blob 交易。keystore 账户必须已经在进程里解开。见 [accounts](../../accounts/LEARNING.md)。

## eth_gasPrice 和 eth_estimateGas

这两条不走出块路径。改了二维 gas 或 base fee 之后，池和 EVM 可以是对的，RPC 给钱包的数字仍会单独错。

`EthereumAPI.GasPrice` 调 `SuggestGasTipCap`，再把当前头的 base fee 加进去。`MaxPriorityFeePerGas` 只返回 tip。实现在 `eth/gasprice`：默认看最近 `FullNodeGPO.Blocks`（20）个块，每块取不低于 `IgnorePrice`（默认 2 wei）的最低 3 笔 tip，跳过提议者自己发的交易，再按 `Percentile`（60）取值。结果封顶 `DefaultMaxPrice`（500 Gwei）。

`BlockChainAPI.EstimateGas` 进 `DoEstimateGas`，再进 `eth/gasestimator.Estimate`。它在指定块的状态上反复执行这笔 call，二分搜索能成功的最小 gas limit。上界先取调用方给的 gas limit，没给则用区块 gas limit。这一次跑通后，下界取 `UsedGas - 1`，并用 `(MaxUsedGas + CallStipend) * 64 / 63` 试一个乐观上界。`ErrorRatio` 大于 0 时，区间相对宽度小于这个比例就停，允许往高估一点。Osaka 且还没到 Amsterdam 时，上界再被 `MaxTxGas` 截断。返回值仍是交易要填的那一个 gas limit；execution / state 二维预算在这次执行内部发生。`Charge` 或 `Forward` 变了，哪些 limit 算成功就会变，估计值跟着变。

`eth_feeHistory` 走同一个 gasprice oracle，但区块数和 reward 百分位来自请求，只用 `MaxHeaderHistory` / `MaxBlockHistory` 截断，不用 `eth_gasPrice` 那套 20 个块、百分位 60 的 tip 采样。

## 已注册的 namespace

| Namespace | 来源 |
|-----------|------|
| eth | `ethapi.GetAPIs`、`eth.APIs`、`filters.NewFilterAPI` |
| txpool | `ethapi.GetAPIs` |
| debug | `ethapi.DebugAPI`、`eth.DebugAPI`、`tracers.API`，以及 node 的 pprof |
| miner / admin / net | `eth.APIs` |
| admin / debug / web3 | `node.Node` |
| engine | `eth/catalyst.Register`，需 JWT。同一次注册还有未认证的 `testing` |

本树没有 `personal_*` RPC。

## 扩展点

现有行为加在对应 API 类型的导出方法上。新的链查询加到 `Backend` 和 `EthAPIBackend` 两边。公开到 HTTP 还要改 `--http.api`。

## 读的时候记住

- `Backend` 是接口。测试可以换成假实现。
- `eth_accounts` 只列地址，不解锁。
- 过滤器订阅要走 WebSocket 或 IPC。
