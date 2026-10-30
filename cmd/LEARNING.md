# cmd：进程入口

[返回学习总览](../LEARNING.md) · 上一篇 [crypto](../crypto/LEARNING.md) · 下一步 [node](../node/LEARNING.md)

`cmd/geth` 是 urfave/cli 外壳。默认命令把命令行和可选 TOML 收成一份配置，构造 `node.Node`，注册以太坊服务，然后阻塞到进程退出。链本身在 [eth](../eth/LEARNING.md) 和 [node](../node/LEARNING.md) 里。

## 建议阅读顺序

1. `cmd/geth/main.go`：`init`、`geth`、`startNode`。
2. `cmd/geth/config.go`：`gethConfig`、`loadBaseConfig`、`makeConfigNode`、`makeFullNode`。
3. `cmd/utils/flags.go`：`SetNodeConfig`、`SetEthConfig`、`RegisterEthService`。
4. `cmd/utils/cmd.go` 的 `StartNode`：真正调用 `stack.Start` 并处理信号。
5. 需要时再读 `consolecmd.go`、`accountcmd.go`。

## 启动调用链

无子命令的 `geth`：

1. `main` → `app.Run`。`Before` 里做 `maxprocs`、全局 flag 迁移、`debug.Setup`、环境变量检查。环境变量前缀是 `GETH`。
2. `geth`：`prepare` → `makeFullNode` → `startNode` → `stack.Wait`。
3. `makeConfigNode`：默认值来自 `ethconfig.Defaults`、`defaultNodeConfig`、`metrics.DefaultConfig`。有 `--config` 时 TOML 覆盖默认值，然后 `SetNodeConfig` 写入显式 CLI。接着 `node.New`（此时会锁数据目录）。账户后端装好之后才 `SetEthConfig`，因为 `--dev` 要解锁账户。
4. `makeFullNode`：分叉覆盖写入 `cfg.Eth.Override*`，然后 `RegisterEthService`。
5. `RegisterEthService` 调用 `eth.New`。数据库、区块链、交易池在这里建好，并把 eth 注册成协议、API 和 `Lifecycle`。失败会退出，返回值是全节点。
6. 随后注册 filter API；可选 GraphQL、ethstats。共识三选一：
   - `--dev`：模拟信标，并 `RegisterLifecycle`。
   - `--beacon.api`：进程内轻客户端 `blsync`。
   - 否则：`catalyst.Register` 挂上要 JWT 的 `engine`，并额外注册未认证的 `testing` 命名空间。这条路径不注册 Lifecycle，节点不会自己按槽出块。
7. `cmd/geth/main.go` 的 `startNode` 调用 `utils.StartNode`。后者 `stack.Start()`，并另起 goroutine 等 SIGINT/SIGTERM。

配置顺序是代码默认值，然后 TOML，然后显式写在命令行上的 flag。多数 setter 用 `ctx.IsSet`，所以没写在命令行上的 flag 不会盖掉 TOML。

## 子命令差异

| 命令 | 行为 |
|------|------|
| `geth console` | 同样 `makeFullNode` 并启动，再用 `stack.Attach()` 进 JS 控制台 |
| `geth attach` | 不建节点，只拨 IPC 或给定端点 |
| `geth account` / `wallet` | 直接操作 keystore，不经过 `node.Node` |
| `geth dumpconfig` | `makeConfigNode` 后吐 TOML。会占用数据目录锁，不启动网络。输出里把 `Genesis` 置空，磁盘上的创世数据不动 |

## 其它可执行文件

| 工具 | 作用 |
|------|------|
| `cmd/evm` | 离线 EVM。`transition` / `t8n` 做单块状态转换 |
| `cmd/devp2p` | 发现与 RLPx 调试，含对活节点的 eth/snap 测试 |
| `cmd/abigen` | 由 ABI 生成 Go 绑定，见 [accounts](../accounts/LEARNING.md) |
| `cmd/ethkey` | 密钥文件的生成、查看、改密、签名 |
| `cmd/rlpdump` | RLP 十六进制与可读结构互转 |
| `cmd/blsync` | 独立信标轻客户端，用 JWT 调外部 Engine API |
| `cmd/keeper` | 读入 payload，做无状态执行并核对根。执行在 `core/stateless` |
| `cmd/fetchpayload` | 从节点拉区块和 execution witness，写成 `keeper` 能读的 payload |
| `cmd/abidump` | 用 `signer/fourbyte` 把一段 calldata 解成函数签名 |
| `cmd/workload` | 对已有 RPC 跑历史、过滤、trace、证明类负载 |
| `cmd/era` | era1 历史文件的读写和校验 |

## 无状态重放

节点正常插块不走这条路径。`cmd/fetchpayload` 向一个在跑的节点要 `debug_executionWitness`，连同区块写成 `cmd/keeper` 能读的 payload。`keeper` 调用 `core.ExecuteStateless`：`Witness.MakeHashDB` 把证明里的 trie 节点、代码和区块哈希放进一块内存 hashdb，再在这份库上执行该块，返回算出来的状态根和收据根，由调用方核对。

证明按哈希寻址。垃圾节点在展开 trie 时对不上哈希。`MakeHashDB` 的注释写明它目前是 hashdb，不是 pathdb。全节点自己的 `InsertChain` 和 Engine API 的 `newPayload` 使用数据目录里的 triedb，不读这份 witness。

## 扩展点

- 新 flag 定义在 `cmd/utils/flags.go`，挂进 `main.go` 的 `nodeFlags` 或 `rpcFlags`。节点侧写入 `SetNodeConfig`，协议侧写入 `SetEthConfig`。要进 TOML 就加到 `gethConfig` 或现有 Config 的导出字段。
- 新后台服务在 `makeFullNode` 里 `RegisterLifecycle`，且必须发生在 `Start` 之前。
- tracer 靠 `main.go` 里对 `eth/tracers/js|live|native` 的空白 import，在 `init` 注册。

## 读的时候记住

- 有两个名字接近的函数：`cmd/geth/main.go` 的 `startNode` 先转调 `utils.StartNode`，再订钱包事件。`cmd/utils/cmd.go` 的 `StartNode` 才调用 `stack.Start`。
- `eth.New` 已经建好链和交易池。`(*Ethereum).Start` 启动发现、handler、连接淘汰 dropper、shutdown 标记和 filtermaps。
- HTTP 和 WebSocket 默认关闭。IPC 默认开启。`defaultNodeConfig` 把模块列表加上 `eth`，只是允许列表，不代表已经监听。
- `--dev` 且不给 `--datadir` 时链在内存里。keystore 仍会落在临时目录，进程退出就删。
