# eth/protocols：eth 与 snap 线协议

[返回 eth](../LEARNING.md) · [返回学习总览](../../LEARNING.md) · 相关 [p2p](../../p2p/LEARNING.md)、[eth/downloader](../downloader/LEARNING.md)

`eth/protocols` 定义执行层节点之间的 devp2p 子协议。当前有两个：`eth` 传区块历史和交易，`snap` 传状态。握手之后的消息循环在各自的 `handler.go`。同步策略在 [downloader](../downloader/LEARNING.md)。

## 建议阅读顺序

1. `eth/protocols/eth/protocol.go`：版本号和消息号。
2. `eth/protocols/eth/handshake.go`：`StatusPacket`。
3. `eth/protocols/eth/handler.go`：`eth69` 到 `eth72` 的分发表。
4. `eth/protocols/eth/handlers.go`：每个消息怎么回答。
5. `eth/protocols/snap/protocol.go`：snap/1 与 snap/2。
6. `eth/protocols/snap/handler.go` 的 `MakeProtocols`。
7. `eth/protocols/snap/sync.go`（snap/1）和 `syncv2.go`（snap/2）。状态同步的算法在这里，调度在 downloader。snap/2 的三个阶段见 [snap](snap/LEARNING.md)。

`eth/handler.go` 的 `runEthPeer` 在连接建立后跑 eth 的 `Run`。对端也宣告了 snap 时，先等 snap 扩展登记，再把 peer 交给 downloader。

## eth

`ProtocolVersions` 是 `ETH72`、`ETH71`、`ETH70`、`ETH69`，优先用 72。消息长度分别是 22、20、18、18。单条消息上限 `maxMessageSize` 是 10 MiB。

`StatusMsg`（`0x00`）核对 networkID、genesis、forkID 和区块范围。对不上就断开。

| 版本 | 在 69 的基础上多出来的 |
|--|--|
| eth69 | 交易、头、body、收据、池内交易、`BlockRangeUpdate` |
| eth70 | 收据编码不同，处理函数换成 `handleReceipts70` |
| eth71 | `GetBlockAccessLists` / `BlockAccessLists` |
| eth72 | 再加 `GetCells` / `Cells`，池内哈希公告改用 72 的结构 |

`NewBlockHashesMsg`（`0x01`）和 `NewBlockMsg`（`0x07`）仍定义在 `protocol.go`。`eth69` 到 `eth72` 的分发表里没有它们。收到会落到 `errInvalidMsgCode`。合并之后的新块从共识层经 [catalyst](../catalyst/LEARNING.md) 进来。

交易路径：`Transactions` 直接带交易体；`NewPooledTransactionHashes` 只带哈希，对端再 `GetPooledTransactions`。eth72 的 blob 用 cell 消息补齐。这些包交给 `eth/fetcher`，而不是在协议层直接写链。

## snap

`ProtocolName` 是 `snap`。版本是 `SNAP2`、`SNAP1`。snap/1 有 8 个消息：账户区间、存储区间、字节码、trie 节点，各有请求和响应。snap/2 再加 `GetAccessLists` / `AccessLists`（`0x08` / `0x09`），消息数是 10。

`MakeProtocols(backend, snapV2)` 在 `snapV2` 为真时才宣告 snap/2。注释说明 snap/2 还不能无条件广告。

snap 不传区块，也不传交易。snap/1 按账户哈希区间拉叶子，再用 trie 节点把缺的枝补上。snap/2 拉扁平账户、存储和字节码，并用区块访问列表把 pivot 之后的状态差额补上；`syncerV2` 的 `OnTrieNodes` 是空操作。downloader 在构造时二选一：`NewV2Syncer` 或 `NewV1Syncer`。

## 读的时候记住

- 新区块不从 eth 协议广播进来。消息号还在，处理函数不在分发表里。
- eth 传链数据和交易。snap 传状态。两边都要有，snap 同步才能完成。
- 对外宣告的版本看 `protocol.go` 的 `ProtocolVersions`。某个消息会不会被处理，看 `handler.go` 的分发表。`NewBlock` 常量还在，不表示还会分发。
- snap/2 是否出现在线路上，取决于 `MakeProtocols` 的 `snapV2` 参数。
