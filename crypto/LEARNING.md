# crypto：哈希、地址与签名

[返回学习总览](../LEARNING.md) · 上一篇 [rlp](../rlp/LEARNING.md) · 下一步 [cmd](../cmd/LEARNING.md)

业务代码只调用 `crypto` 包的公开函数。`crypto/secp256k1/libsecp256k1` 是上游 C 库，读协议时停在 Go 封装即可。

## 建议阅读顺序

1. `crypto/crypto.go`：Keccak、`Sign`、`Ecrecover`、`CreateAddress`。
2. `crypto/secp256k1` 的 Go 文件：cgo 薄封装。
3. `crypto/signature_nocgo.go`：nacl、js、wasip1、无 cgo、gofuzz 或 tinygo 时改用纯 Go 的 secp256k1。两边函数签名一致。

## 公开 API

**Keccak。** `Keccak256`、`Keccak256Hash`、`NewKeccakState`。这是以太坊的 Legacy Keccak-256，实现在 `crypto/keccak`，带 hasher 池。它和 NIST SHA3-256 的填充不同，两者的结果不能互换。

**地址。**

- `PubkeyToAddress`：去掉未压缩公钥的 `0x04` 后再 keccak，取后 20 字节。
- `CreateAddress`：`keccak(rlp([sender, nonce]))` 的后 20 字节。
- `CreateAddress2`：`keccak(0xff ++ sender ++ salt ++ keccak(initcode))`，再取后 20 字节。入参 `inithash` 已经是 initcode 的哈希。

**密钥。** `GenerateKey`、`HexToECDSA`、`FromECDSA`、`UnmarshalPubkey`。`LoadECDSA` / `SaveECDSA` 读写磁盘上的密钥文件。`cmd/ethkey` 和 keystore 在更高一层，见 [accounts](../accounts/LEARNING.md)。

**签名。** `Sign(digestHash, priv)` 要求摘要正好 32 字节，输出 65 字节 `[R || S || V]`，`V` 为 0 或 1。`Ecrecover` / `SigToPub` 从签名恢复公钥。`VerifySignature` 校验 64 字节 `[R || S]`，不含 V。`ValidateSignatureValues` 检查 r、s 从 1 到 N−1（含 1，不含曲线阶 N）、v 为 0 或 1，Homestead 起还要求 s 不超过半阶。交易里带 chain ID 的 `V`（EIP-155）在 `core/types` 的 `Signer`，不在这个包。

## 扩展点

新的哈希用途直接调用 `Keccak256Hash`。仍是 secp256k1 上对 32 字节摘要签名时，复用 `Sign` / `Ecrecover`。预编译里的 bn256、BLS、secp256r1 在 `core/vm/contracts.go`，走各自的库。

## 读的时候记住

- 传给 `Sign` 的是哈希，函数内部不会再做一次 keccak。
- `crypto` 不管理口令。解锁在 [accounts/keystore](../accounts/LEARNING.md)。
