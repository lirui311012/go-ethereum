# common：地址、哈希与十六进制

[返回学习总览](../LEARNING.md) · 相关 [crypto](../crypto/LEARNING.md)、[rlp](../rlp/LEARNING.md)、[rpc](../rpc/LEARNING.md)

`common` 放全仓库共用的值类型。区块、交易、状态根用 `Hash`；账户用 `Address`。哈希运算在 [crypto](../crypto/LEARNING.md)。区块和交易的二进制编码在 [rlp](../rlp/LEARNING.md)。

## 建议阅读顺序

1. `common/types.go`：`Hash`、`Address`、`SetBytes`、`Hex`、`MixedcaseAddress`。
2. `common/bytes.go`：`FromHex`、`LeftPadBytes`、`RightPadBytes`。
3. `common/hexutil`：`Encode`、`Decode`、`Big`、`Uint64`。JSON-RPC 的 `0x` 字符串走这里。
4. `common/math/big.go`：`HexOrDecimal256`、`U256`、`PaddedBigBytes`。配置文件里的大整数经常是这个类型。

## Hash 与 Address

`Hash` 是 `[32]byte`，`Address` 是 `[20]byte`。常量是 `HashLength` 和 `AddressLength`。

`BytesToHash` / `BytesToAddress` 都从零值出发再调用 `SetBytes`。输入比目标长时，从左侧裁掉，留下右边那一段。短输入写到数组尾部，左边保持 0，看起来像靠左补零。直接对已经有内容的值调用 `SetBytes` 不会清掉左边。这和 EVM 里把数值放在 32 字节字的右侧是同一习惯。

`Address.Hex` 和 `String` 输出 EIP-55 校验和：对地址的小写十六进制做一次 keccak，哈希里对应半字节大于 7 时把该字符改成大写。`Hash.Hex` 只做 `0x` 加小写十六进制，没有校验和。

`MixedcaseAddress` 保留调用方原来的大小写，`ValidChecksum` 用来核对它是不是合法的 EIP-55。`AddressEIP55` 在 JSON 里强制输出校验和形式。

JSON 解码走 `hexutil.UnmarshalFixedJSON`，字节数必须刚好是 32 或 20。多一个字节或少一个字节都会失败；十六进制少写一个字符也会因为长度不是偶数而失败。裁剪只发生在 `SetBytes`。编码走 `MarshalText`，是小写 `0x`。

## 和 RLP、RPC 的差别

| | 用在哪 | 入口 |
|--|--|--|
| `common/hexutil` | JSON-RPC、日志里给人看的 `0x` | `hexutil.Encode` / `Decode` |
| `rlp` | 区块、交易、收据在磁盘和 devp2p 上的二进制 | `rlp.Encode` / `Decode` |

同一份 `Hash` 可以有两种外观：RPC 响应是 `0x` 字符串，块头 RLP 里是 32 个原始字节。

## 旁边的小包

- `common/math`：`ParseBig256` 同时接受十进制和 `0x`。`U256` 把整数折进 256 位。`SafeAdd` / `SafeMul` 在 `integer.go`。
- `common/mclock`：单调时钟。`System` 用真实时间，测试里用 `Simulated`。P2P 和请求调度靠它，不靠 `time.Now` 做间隔。
- `common/lru`：泛型 `Cache`。`SizeConstrainedCache` 按字节而不是按条目数淘汰，blob 缓存用它。
- `common/prque`：带优先级的队列。下载器和同步里排序用。
- `common/bitutil`：字节切片的与、或、异或，以及稀疏位图压缩。
- `common/fdlimit`：按操作系统提高进程能打开的文件数。`node` 启动时调用。

`StorageSize` 在 `size.go`，按字节格式化成 B、KiB、MiB、GiB 或 TiB，只给日志看。

## 读的时候记住

- `HexToAddress("0x1")` 得到的是末尾为 `0x01` 的 20 字节地址，前面补零。
- `SetBytes` 裁的是左边。把一个 32 字节哈希塞进 `Address` 会丢掉高 12 字节。
- EIP-55 出现在 `Address.Hex`、`String`、`%v`、`%s` 和 `%q`。`%x` / `%X` 没有校验和。`MarshalText` 以及因此发生的 `json.Marshal` 是小写 `0x`。要在 JSON 里带校验和，用 `AddressEIP55`。比较地址用字节。
- `hexutil.Decode` 要求 `0x` 前缀。`common.FromHex` 对没有前缀的字符串更宽容。
