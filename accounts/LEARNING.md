# accounts：钱包、keystore 与 ABI

[返回学习总览](../LEARNING.md) · 上一篇 [internal/ethapi](../internal/ethapi/LEARNING.md)

`accounts` 把「谁来签名」和「合约 ABI 怎么编解码」放在一起。签名算法在 [crypto](../crypto/LEARNING.md)。RPC 会调用 `Wallet.SignTx` 和 `Wallet.SignText`。`eth_sign` 走 `SignText`，见 [internal/ethapi](../internal/ethapi/LEARNING.md)。

## 建议阅读顺序

1. `accounts/accounts.go`：`Wallet`、`Backend`、`Account`。
2. `accounts/manager.go`：按地址 `Find` 到钱包。
3. `accounts/keystore/keystore.go`：`Unlock`、`TimedUnlock`、`SignTx`、`SignTxWithPassphrase`。
4. `accounts/usbwallet`、`accounts/scwallet`：硬件上签名。
5. `accounts/external`：外部签名器 URL。`cmd/geth/config.go` 的 `setAccountManagerBackends` 在配置了它时不再加载本地 keystore。
6. `accounts/abi`：`Pack` / `Unpack`。详解 [accounts/abi](abi/LEARNING.md)。
7. `accounts/abi/abigen`：生成器。`cmd/abigen/main.go` 是入口。

## 解锁与签名

解锁只在 keystore。`Unlock` / `TimedUnlock` 把解密后的私钥放进内存 `unlocked`。`SignTx` 找不到就返回 `ErrLocked`。`SignTxWithPassphrase` 当场解密、签完清零，不依赖事先解锁。

本树没有 `personal_unlockAccount`。`geth account`（`cmd/geth/accountcmd.go`）直接操作 keystore，不走 RPC。

硬件钱包在设备上签，没有 keystore 解锁。外部签名器与本地 keystore、USB、智能卡互斥。

`eth_sendRawTransaction` 不经过钱包。

## abigen

生成器在树里，生成结果默认不在树里。

- v1：`accounts/abi/abigen.Bind`，模板在 `template.go`。运行时 `accounts/abi/bind` 已标废弃。
- v2：`BindV2`，模板是 `source2.go.tpl`，由 `template.go` 嵌入。运行时是 `accounts/abi/bind/v2`。`--v2` 走这条。
- 产物写到 `--out` 或标准输出。仓库里的样例在 `accounts/abi/bind/v2/internal/contracts` 的测试夹具。

## signer 目录

本树没有 `cmd/clef`。留下的是库：`signer/fourbyte`（选择器）、`signer/core/apitypes`（EIP-712）、`signer/storage`。详解 [signer](../signer/LEARNING.md)。

## 读的时候记住

- `node.New` 只创建空的 `Manager`。后端在 geth 启动时装上。
- keystore 目录在 `DataDir/keystore`，同一数据目录的多个实例共享。`--dev` 且无 datadir 时用临时目录，进程退出删除。
- 改 ABI 绑定优先走 abigen v2，少手改 `bind` 运行时。
