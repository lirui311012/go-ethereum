# core/types：区块、交易与收据

[返回 core](../LEARNING.md) · [返回学习总览](../../LEARNING.md) · 相关 [rlp](../../rlp/LEARNING.md)、[core/rawdb](../rawdb/LEARNING.md)

`core/types` 定义链上数据的形状，以及怎么算哈希、怎么签名。它不插入区块，也不执行交易。那两步在 [core](../LEARNING.md)。

## 建议阅读顺序

1. `core/types/block.go`：`Header`、`Body`、`Block`、`Header.Hash`。
2. `core/types/transaction.go`：五种交易类型，以及 `Hash`、`WithBlobTxSidecar`。
3. `core/types/tx_legacy.go`、`tx_access_list.go`、`tx_dynamic_fee.go`、`tx_blob.go`、`tx_setcode.go`：每种 `TxData`。
4. `core/types/transaction_signing.go`：`MakeSigner`、`LatestSigner`、`Sender`。
5. `core/types/receipt.go`：`Receipt` 和 `storedReceiptRLP`。
6. `core/types/log.go`、`bloom9.go`、`withdrawal.go`。

`gen_header_json.go`、`gen_header_rlp.go` 这类 `gen_*.go` 由 `gencodec` / `rlpgen` 生成。改结构体后跑旁边的 `go:generate`，不要手改生成文件。

## 块头

`Header.Hash` 是整份头 RLP 的 keccak256。共识字段包括 `ParentHash`、`Root`、`TxHash`、`ReceiptHash`、`Bloom`、`Difficulty`、`Number`、`GasLimit`、`GasUsed`、`Time`。

后面的字段带 `rlp:"optional"`，旧块没有它们：

| 字段 | 来源 |
|--|--|
| `BaseFee` | EIP-1559 |
| `WithdrawalsHash` | EIP-4895 |
| `BlobGasUsed`、`ExcessBlobGas` | EIP-4844 |
| `ParentBeaconRoot` | EIP-4788 |
| `RequestsHash` | EIP-7685 |
| `BlockAccessListHash` | EIP-7928 |
| `SlotNumber` | EIP-7843 |

`Body` 含交易、叔块和提款。`Block` 在头和 body 之上缓存哈希。Amsterdam 的区块访问列表另外挂在块上，协议里的 `extblock` 编码不含它。

## 交易

`Transaction` 是外壳加 `inner TxData`。类型常量：

| 常量 | 值 | 内层类型 |
|--|--|--|
| `LegacyTxType` | `0x00` | `LegacyTx` |
| `AccessListTxType` | `0x01` | `AccessListTx` |
| `DynamicFeeTxType` | `0x02` | `DynamicFeeTx` |
| `BlobTxType` | `0x03` | `BlobTx` |
| `SetCodeTxType` | `0x04` | `SetCodeTx` |

传统交易直接 RLP。类型化交易是 `类型字节 || RLP(payload)`。`Hash` 对这个共识编码做 keccak，结果缓存在 `hash` 里。

Blob 交易的 sidecar（blob 和 KZG 证明）在进交易池、在节点之间传时可以带着。写进区块的那一份要先 `WithoutBlobTxSidecar`。`Hash` 算的是不包含 sidecar 的交易体，所以摘掉 sidecar 不改变交易哈希。

## 签名

`MakeSigner(config, number, time)` 按区块所在分叉选签名器：Prague、Cancun、London、Berlin、EIP-155、Homestead、Frontier。验证历史交易用它。

`LatestSigner` 在还不知道具体高度时，按 `ChainConfig` 里已经排期的最晚分叉选一个最宽的签名器。交易池收新交易时用它。`LatestSignerForChainID` 只拿到 chain ID 时用。

`Sender` 从签名恢复地址，并缓存在交易上。链配置变了要换签名器，缓存会失效后重算。

## 收据

共识部分是类型、`PostState` 或 `Status`、累计 gas、Bloom、日志。`TxHash`、合约地址、`GasUsed`、有效 gas 价格、blob gas 是实现侧字段，JSON-RPC 会返回，共识 RLP 不包含全部。

Byzantium 之前用 `PostState`（当时的状态根）。之后用 `Status`：`ReceiptStatusSuccessful` 是 1，`ReceiptStatusFailed` 是 0。落盘的 `storedReceiptRLP` 把二者合成 `PostStateOrStatus`，并且不存 Bloom。

## 读的时候记住

- 块哈希是头的 RLP 哈希。改任何一个共识字段，哈希都会变。
- 进块的 blob 交易没有 sidecar。池里和网络上的可以有。
- `MakeSigner` 要区块高度和时间。`LatestSigner` 用于还没有这两者的路径。
- 收据里的 `Status` 和 `PostState` 在存储里是同一个字段的两种解释。
