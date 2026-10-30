# ethclient：用 Go 调用一个节点

[返回学习总览](../LEARNING.md) · 相关 [rpc](../rpc/LEARNING.md)、[internal/ethapi](../internal/ethapi/LEARNING.md)

`ethclient` 是库。它通过 JSON-RPC 问一个已经在跑的节点：区块、余额、收据、发交易。节点怎么执行这些调用，在 [internal/ethapi](../internal/ethapi/LEARNING.md)。传输层在 [rpc](../rpc/LEARNING.md)。

## 建议阅读顺序

1. `ethclient/ethclient.go`：`Dial`、`NewClient`、`Client`。
2. 同一文件里按用途跳：`BlockByNumber`、`BalanceAt`、`CallContract`、`SendTransaction`、`SubscribeNewHead`。
3. `ethclient/gethclient/gethclient.go`：只在 geth 上有的扩展命名空间。
4. `ethclient/simulated/backend.go`：测试里用的进程内节点。

## 连接

`Dial` / `DialContext` 接受 `http`、`ws`、`ipc` 地址，内部交给 `rpc.Dial`。`NewClient` 则包一层已经建好的 `*rpc.Client`。`Close` 关掉底层连接。

`BlockByNumber(nil)` 表示最新块，实现里把 `nil` 的 `*big.Int` 译成 RPC 标签 `latest`。查历史状态时传入那个高度。按哈希查用 `BalanceAtHash`、`StorageAtHash`、`NonceAtHash` 这一组。

`SendTransaction` 把已签名的 `types.Transaction` 编成 RLP，调用 `eth_sendRawTransaction`。签名要在调用前完成，这个包不保管私钥。`ethclient/signer.go` 里的 `senderFromServer` 只把 RPC 已经返回的发送者地址放进缓存，`Hash` 和 `SignatureValues` 会 panic。

`CallContract` 走 `eth_call`，不进交易池。`EstimateGas` 走 `eth_estimateGas`。

订阅需要 websocket 或 IPC。`SubscribeNewHead`、`SubscribeFilterLogs` 在 HTTP 上会失败，因为 HTTP 没有服务端推送。返回的 `ethereum.Subscription` 用 `Err()` 观察断线。

## 模拟后端

`ethclient/simulated` 在进程里拉起 `node.Node` 和 `eth.Ethereum`，并用 catalyst 的模拟信标出块。测试可以 `Commit` 出块再读取。对外的 `Client` 接口是 `ethclient` 的一个子集，`SimulateV1`、`SendTransactionSync` 这些方法不在上面。

`accounts/abi/bind/backends` 里的旧模拟后端已经标成废弃，注释指向这个包。

`gethclient` 和标准 `ethclient` 分开：前者依赖 geth 私有命名空间，换一个执行客户端时这些方法可能不存在。

## 读的时候记住

- 这个包只组 RPC 请求。打开数据库和执行 EVM 的是节点进程。
- `nil` 区块号是 `latest`。要查某一高度，把 `*big.Int` 传进去。
- 订阅使用 websocket 或 IPC。
- 模拟后端会跑链和 Engine API，只是不出进程、不连主网。
