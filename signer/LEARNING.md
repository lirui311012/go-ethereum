# signer：选择器数据库、EIP-712 与加密存储

[返回学习总览](../LEARNING.md) · 上一篇 [accounts](../accounts/LEARNING.md) · 相关 [crypto](../crypto/LEARNING.md)

当前仓库里没有 `cmd/clef`。留下的是三块库：4 字节选择器、EIP-712 结构化数据、用 AES-GCM 加密的小文件存储。账户解锁和 `Wallet.SignTx` 在 [accounts](../accounts/LEARNING.md)。

## 建议阅读顺序

1. `signer/fourbyte/fourbyte.go`：`Database`、`New`、`NewWithFile`。
2. `signer/fourbyte/validation.go`：对 calldata 做的检查。
3. `signer/core/apitypes/types.go`：`TypedData`、`HashStruct`、`EncodeData`。
4. `signer/storage/aes_gcm_storage.go`：`AESEncryptedStorage`。

## 4 字节选择器

合约调用的前 4 字节是函数签名 keccak 的前 4 字节，和 [accounts/abi](../accounts/abi/LEARNING.md) 里 `Method.ID` 是同一种算法。

`fourbyte.New` 载入包内嵌的 `4byte.json`，把这 4 字节映回人类可读的函数签名。`NewWithFile` 再叠加一份可以写回磁盘的自定义表。嵌入的表不改，自定义表存在调用方给的路径。

`validation.go` 用这张表说明这段 calldata 对得上哪个签名，以及参数打包是否和签名对得齐。它不执行交易。

## EIP-712

`TypedData` 有 `Types`、`PrimaryType`、`Domain`、`Message`。`HashStruct` 是 `keccak(EncodeData)`，只覆盖一个结构。真正要签的 32 字节是 `TypedDataAndHash`：`keccak("\x19\x01" || domainSeparator || hashStruct)`。

这个摘要再交给 [crypto](../crypto/LEARNING.md) 的 secp256k1。

## 存储

`storage.AESEncryptedStorage` 把 JSON map 加密后写到一个文件。密钥由调用方提供。这是给签名器记「用户曾经批准过什么」用的。keystore 的文件格式在 `accounts/keystore`。

## 读的时候记住

- 本树没有 Clef 命令行入口。这里是库。
- 4 字节数据库把 calldata 映回签名。ABI 的 `Pack` 把参数编成 calldata。
- 要签的是 `TypedDataAndHash`，不是单独的 `HashStruct`。签名在 `crypto`。
- 加密存储的密钥不在文件里。keystore 的口令流程是另一套。
