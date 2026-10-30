# params：链配置与协议常数

[返回学习总览](../LEARNING.md) · 下一步 [rlp](../rlp/LEARNING.md)

`params` 回答一个问题：这条链在这个高度启用哪套规则。同一份代码靠数据库里的 `ChainConfig` 区分主网、测试网和 `--dev`。

## 建议阅读顺序

1. `params/config.go`：`ChainConfig`、`Rules`、`MainnetChainConfig`、`AllDevChainProtocolChanges`。
2. `params/protocol_params.go`：gas、区块上限等常数。EVM 和状态转换引用这里的数字，少在调用点写魔数。
3. `params/forks`：分叉枚举，`ChainConfig.LatestFork` 用来算当前最新的时间分叉。
4. 需要地址和哈希时读 `common/types.go`；需要 JSON-RPC 的 `0x` 编码时读 `common/hexutil`。详解 [common](../common/LEARNING.md)。
5. 需要进程内事件时读 `event/feed.go`。链头、日志从 `core.BlockChain` 的 `Feed` 送到 `eth/filters`。详解 [event](../event/LEARNING.md)。

## ChainConfig 与 Rules

Merge 之前的分叉用区块号（`*big.Int`）。`nil` 表示不启用，`0` 表示创世已经启用。Shanghai 起改成时间戳（`*uint64`）。The Merge 本身看 `TerminalTotalDifficulty`，由调用方把 `isMerge` 传进 `Rules`。

`IsHomestead`、`IsShanghai` 这类方法只回答「这个高度是否已经过了该分叉」。`Rules(num, isMerge, timestamp)` 把布尔值收成一次性快照，交给执行层。`core/vm.NewEVM` 用它选 jump table 和预编译，之后换交易只换 `TxContext`。同一笔交易进入 `execute` 时还会再算一次 `Rules`，给 preCheck 和内在 gas 用。Shanghai 及之后的规则还要求已经 Merge。

改已经激活的分叉号时，`CheckCompatible` 会要求回退链头。主网、Sepolia、Hoodi 的激活时间写在对应的 `*ChainConfig` 里。

| 配置 | Chain ID | 用途 |
|------|----------|------|
| `MainnetChainConfig` | 1 | 主网 |
| `SepoliaChainConfig` | 11155111 | 公共测试网 |
| `HoodiChainConfig` | 560048 | 公共测试网 |
| `AllDevChainProtocolChanges` | 1337 | `--dev`。时间分叉开到 Bogota。Amsterdam、UBT、BPO1、BPO2 未设置 |

`NetworkNames` 把 chain ID 映成 `mainnet` / `sepolia` / `hoodi`。

## common 与 hexutil

`common.Hash` 是 `[32]byte`，`common.Address` 是 `[20]byte`。`BytesToHash` / `BytesToAddress` 超长时从左侧截掉。`Hash.Hex` 和两边的 JSON 走 `hexutil`，带小写 `0x`。`Address.Hex` 是 EIP-55 校验和；`Address` 的 JSON 仍是小写 `0x`。

`hexutil` 是 JSON-RPC 规则，和 [RLP](../rlp/LEARNING.md) 不同。字节串必须偶数长度，空串是 `"0x"`。整数去掉前导零，零是 `"0x0"`。`Hash` 和 `Address` 用定长解码，少一个字节会失败。

## event

`event.TypeMux` 已废弃。新代码用 `Feed` 或泛型 `FeedOf[T]`。`Subscribe` 传入可发送的 channel；`Send` 把同一个值交给当前所有订阅者。慢订阅者不会被丢掉，channel 要自己留缓冲。类型在第一次 `Send` 或 `Subscribe` 时锁定。`Unsubscribe` 必须调用。

`core.BlockChain` 持有链头、日志、移除日志等 feed。`eth/filters.EventSystem` 订阅它们，再转成 `eth_subscribe`。

## Fork ID

`core/forkid` 实现 EIP-2124。eth 握手的 `StatusPacket` 带上 `forkid.NewID` 的结果，对端用 `NewFilter` 决定要不要断开。

`ID.Hash` 是 CRC32：从创世哈希开始，把已经过去的分叉块号或时间戳依次混进去。`ID.Next` 是下一个还没到的分叉点，没有下一个就是 0。`gatherForks` 用反射扫 `ChainConfig` 的字段：名字以 `Block` 结尾且类型是 `*big.Int` 的进区块分叉，以 `Time` 结尾且类型是 `*uint64` 的进时间分叉。`nil` 表示未安排，不进校验和。

因此新硬分叉除了 `Rules` 里的布尔值，字段名也要能被这次扫描看到。名字对不上时，本机已经按新规则执行，握手仍报旧的 fork ID，对端会以 `ErrRemoteStale` 或 `ErrLocalIncompatibleOrStale` 断开。时间分叉和区块分叉靠 `timestampThreshold`（主网创世时间）区分 `Next` 里的数字是块号还是时间戳。

## 扩展点

新硬分叉先在 `ChainConfig` 加字段，补 `Is*` 和 `Rules` 里的布尔值，再让 [core/vm](../core/vm/LEARNING.md)、[core](../core/LEARNING.md)、[consensus](../consensus/LEARNING.md) 消费这份快照。激活时间只改各网络的 `*ChainConfig`，执行逻辑读 `Rules`。

## 读的时候记住

- 执行热路径读 `Rules`，少在 opcode 循环里反复查 `ChainConfig`。
- `params` 放「何时启用」和「数字是多少」。opcode 语义在 `core/vm`，头字段校验在 `consensus`。
