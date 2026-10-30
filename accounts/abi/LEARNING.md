# accounts/abi：ABI 编解码与代码生成

[返回 accounts](../LEARNING.md) · [返回学习总览](../../LEARNING.md) · 相关 [signer](../../signer/LEARNING.md)

`accounts/abi` 把 Solidity ABI JSON 变成 Go 结构，并把参数打包成 calldata、把返回值和日志解出来。它不发交易。发送在 [ethclient](../../ethclient/LEARNING.md) 或钱包里。

## 建议阅读顺序

1. `accounts/abi/abi.go`：`JSON`、`Pack`、`Unpack`、`UnpackIntoInterface`。
2. `accounts/abi/method.go`：`NewMethod` 如何计算 `Sig` 和 `ID`。
3. `accounts/abi/type.go`：`NewType`。
4. `accounts/abi/pack.go`、`unpack.go`：静态类型和动态类型怎么排。
5. `accounts/abi/topics.go`：事件 indexed 参数和 topic。
6. `accounts/abi/event.go`：`Event`。
7. `accounts/abi/abigen/bind.go` 与 `bindv2.go`。运行时绑定在 `accounts/abi/bind/v2`。

## 从 JSON 到调用数据

`JSON(reader)` 得到 `ABI`，里面是 `Methods`、`Events`、`Errors`。重载方法的内部名字会加后缀：第一个 `foo` 仍叫 `foo`，第二个变成 `foo0`。`RawName` 保留 JSON 里的原名。

`Pack(name, args...)` 找到方法，把参数编成 ABI 二进制，再在前面加上 4 字节 `ID`。`ID` 只对普通函数计算：`keccak256("transfer(address,uint256)")` 的前 4 字节。签名里的类型名沿用 JSON 里的字符串。不带位宽的 `int` / `uint` 在 `NewType` 里直接报错，不会改写成 `int256`。构造函数、`fallback`、`receive` 没有这个选择器。

`Unpack` 按方法的输出解返回值。解日志时用事件名。`UnpackIntoInterface` 把结果写进结构体，`UnpackIntoMap` 写进 `map[string]interface{}`。

## 类型布局

`NewType` 认识 `uint256`、`address`、`bytes`、`string`、数组、切片和 tuple。单个静态值占一个 32 字节字。定长数组和静态 tuple 按 `getTypeSize` 占多字。`bytes`、`string`、切片和动态 tuple 在原地放偏移，内容追加在尾部。嵌套动态类型会再套一层偏移。细节在 `pack.go`，读的时候拿一笔 `transfer` 和一笔带 `bytes` 参数的调用对比即可。

事件的非 indexed 数据走同一套打包。indexed 参数进 topic：值类型直接放进 32 字节字，动态类型则是值的 keccak。topic0 是事件签名的 keccak，除非事件标了 `anonymous`。`topics.go` 的 `MakeTopics` 负责把 Go 值变成这些 topic。

## abigen

生成器在树里，生成结果默认不在树里。

- v1：`accounts/abi/abigen.Bind`，模板是 `source.go.tpl`（由 `template.go` 嵌入）。运行时 `accounts/abi/bind` 已标废弃。`Deprecated` 写的是 `github.com/ethereum/go-ethereum/bind/v2`。本树的 v2 运行时在 `accounts/abi/bind/v2`。
- v2：`BindV2`，模板是 `source2.go.tpl`。`cmd/abigen` 的 `--v2` 走这条。
- 产物写到 `--out` 或标准输出。

v2 运行时用 `bind/v2` 的部署、调用和事件过滤。旧的 `accounts/abi/bind/backends` 模拟后端废弃，测试改用 [ethclient/simulated](../../ethclient/LEARNING.md)。

`signer/fourbyte` 用同一套 4 字节算法做反向查找：从 calldata 猜函数名。

## 读的时候记住

- `Pack` 的返回值已经含 4 字节选择器。再手动加一次会多出 4 字节。
- 重载方法在 `Methods` 里的键可能是 `foo0`，不是 JSON 里的 `foo`。
- indexed 动态参数的 topic 是哈希，解不回原文。原文在非 indexed 的 data 里。
- 改绑定优先跑 abigen v2，少手改 `bind` 运行时。
