# beacon：执行载荷类型与信标轻客户端

[返回学习总览](../LEARNING.md) · 相关 [consensus](../consensus/LEARNING.md)、[eth/catalyst](../eth/catalyst/LEARNING.md)、[miner](../miner/LEARNING.md)

这个目录描述共识层交给执行层的那一块长什么样，以及一个可选的信标轻客户端。听 Engine API 的服务在 [eth/catalyst](../eth/catalyst/LEARNING.md)。块头规则在 [consensus/beacon](../consensus/LEARNING.md)。

## 建议阅读顺序

1. `beacon/engine/types.go`：`PayloadAttributes`、`ExecutableData`、`ForkchoiceStateV1`、`PayloadID`、`PayloadVersion`。
2. 同一文件里的 `ExecutableDataToBlock` 和 `BlockToExecutableData`。
3. `beacon/params/params.go`：`EpochLength`、同步委员会大小。这是轻客户端用的常数。
4. `beacon/types`：信标头、执行载荷头、轻客户端更新。
5. `beacon/light`：`CommitteeChain`、`HeadTracker`。`beacon/light/sync` 拉 checkpoint 和后续更新。
6. `beacon/blsync/client.go`：`NewClient` 把上面几块接上本机 Engine API。入口命令是 `cmd/blsync`。

## 执行载荷

`PayloadAttributes` 是组下一块时的环境：时间戳、`prevRandao`、建议的手续费接收地址，以及按分叉出现的提款、`parentBeaconBlockRoot`、slot、目标 gas limit。

`ExecutableData` 是已经组好、可以执行的块：父哈希、状态根、交易的二进制、提款、blob gas、区块访问列表、slot。`ExecutableDataToBlock` 把它变成 `types.Block`，并核对区块哈希。`BlockToExecutableData` 走相反方向，附上手续费、blob sidecar 和 execution requests。

`ForkchoiceStateV1` 有三个哈希：`HeadBlockHash`、`SafeBlockHash`、`FinalizedBlockHash`。共识层用它告诉执行层链头、安全点和最终点。

`PayloadID` 是 8 字节。第一个字节是 `PayloadVersion`：V1 Paris、V2 Shanghai（提款）、V3 Cancun（blob gas）、V4 Amsterdam（slot 和区块访问列表）。RPC 方法的版本号和这个字节不是一一对应。`GetPayloadV3`、`V4`、`V5` 都要求 `PayloadV3`；`GetPayloadV6` 才要求 `PayloadV4`。

`ExecutionPayloadEnvelope` 是 `getPayload` 的返回：载荷、总手续费、blobs、requests。具体哪一版 RPC 方法在 catalyst 里分发。

## 轻客户端

`beacon/light` 跟踪同步委员会和信标头，用来确认这个信标头被委员会签过。它不保存执行层状态。

`blsync.NewClient` 在内存库上建 `CommitteeChain` 和 `HeadTracker`，向信标节点的 HTTP 接口要轻客户端更新，再经 `engineClient` 对本机执行层调用 Engine API，把跟到的执行载荷送进去。全节点旁边有完整共识客户端时，由那个进程直接调 catalyst，不经过 `blsync`。

`beacon/merkle` 提供轻客户端证明用的 Merkle 计算。

## 没有共识客户端时谁推链头

两条路径都假定本机没有完整的共识客户端，用途不同。

`beacon/blsync` 连信标节点的轻客户端 HTTP 接口，自己跟踪同步委员会，再对本机 Engine API 送执行载荷。它跟的是信标头，不要求操作者指定一个执行块哈希。

`eth/syncer` 是执行层里的辅助服务。`Syncer.Sync` 收下调用者给的块哈希，经 downloader 向 peer 要到这个头之后，调用 `BeaconDevSync`。配置里的同步模式必须是 full，否则直接返回错误。同步事件到来后，它用当前头往回 `EpochLength` 和 `EpochLength*2` 合成 safe 与 finalized 标记，因为没有共识客户端来提供真的标记。`ExitWhenSynced` 为真时，到达目标后关掉节点。

日常跟主网用的是外部共识客户端调 `eth/catalyst`。`blsync` 和 `eth/syncer` 都不替代那条路径。

## 读的时候记住

- 类型在 `beacon/engine`。JSON-RPC 方法在 `eth/catalyst`。头校验在 `consensus/beacon`。
- `PayloadAttributes` 是组块参数。`ExecutableData` 是已经组好的块。
- `EpochLength` 是 32 个 slot。这里的 epoch 服务于轻客户端校验。
- 全节点日常跟着自己的共识客户端走。没有完整共识客户端时，`blsync` 跟信标头，`eth/syncer` 按给定的执行块哈希做 `BeaconDevSync`。
