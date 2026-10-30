// Copyright 2014 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

// geth is a command-line client for Ethereum.
//
// 不带子命令时进入 geth()：把命令行和可选 TOML 收成配置，建好节点后阻塞到退出。
// 链和交易池在 makeFullNode 里建好。本文件的 startNode 先调用 utils.StartNode，再订钱包插拔事件。
// 真正调用 stack.Start 的是 cmd/utils/cmd.go 的 StartNode。
package main

import (
	"fmt"
	"os"
	"slices"
	"sort"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/console/prompt"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/internal/debug"
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"go.uber.org/automaxprocs/maxprocs"

	// Force-load the tracer engines to trigger registration.
	// 空白导入只为执行这些包的 init，把 tracer 注册进目录。去掉后 debug_trace 找不到对应名字。
	_ "github.com/ethereum/go-ethereum/eth/tracers/js"
	_ "github.com/ethereum/go-ethereum/eth/tracers/live"
	_ "github.com/ethereum/go-ethereum/eth/tracers/native"

	"github.com/urfave/cli/v2"
)

const (
	clientIdentifier = "geth" // Client identifier to advertise over the network
)

var (
	// flag 是带 -- 的命名选项，也就是命令的参数，例如 geth --datadir /data --http。
	// 它和下面 app.Commands 里的子命令不是一类东西：子命令是第一个位置参数
	// （console、account、init），用来选跑哪段逻辑；flag 写在命令前后，用来改配置。
	//
	// 这三组挂到根命令 app.Flags。不写子命令时走 app.Action（geth），这些 flag 全部有效。
	// 子命令不会自动继承这份清单，各自的 Flags 字段决定它认哪些：console 会再拼上
	// nodeFlags 和 rpcFlags，account 只挑自己用得到的。
	// 定义在 cmd/utils/flags.go，这里只负责挂到命令上。加进切片之后，还要在
	// SetNodeConfig 或 SetEthConfig 里写进配置，否则命令行认了也不生效。
	// nodeFlags 管节点和协议，rpcFlags 管对外接口，metricsFlags 管指标。
	nodeFlags = slices.Concat([]cli.Flag{
		// 节点名、解锁密码、引导节点、磁盘下限、keystore、外部签名器和硬件钱包。
		utils.IdentityFlag,
		utils.PasswordFileFlag,
		utils.BootnodesFlag,
		utils.MinFreeDiskSpaceFlag,
		utils.KeyStoreDirFlag,
		utils.ExternalSignerFlag,
		utils.USBFlag,
		utils.SmartCardDaemonPathFlag,
		// 手动改硬分叉时间，或用文件替换内置创世配置。
		utils.OverrideOsaka,
		utils.OverrideAmsterdam,
		utils.OverrideBPO1,
		utils.OverrideBPO2,
		utils.OverrideUBT,
		utils.OverrideGenesisFlag,
		// 普通交易池：本地账户豁免、重启后保留的 journal、价格门槛、可执行槽位和排队槽位。
		utils.TxPoolLocalsFlag,
		utils.TxPoolNoLocalsFlag,
		utils.TxPoolJournalFlag,
		utils.TxPoolRejournalFlag,
		utils.TxPoolPriceLimitFlag,
		utils.TxPoolPriceBumpFlag,
		utils.TxPoolAccountSlotsFlag,
		utils.TxPoolGlobalSlotsFlag,
		utils.TxPoolAccountQueueFlag,
		utils.TxPoolGlobalQueueFlag,
		utils.TxPoolLifetimeFlag,
		// blob 交易池（EIP-4844）。
		utils.BlobPoolDataDirFlag,
		utils.BlobPoolDataCapFlag,
		utils.BlobPoolPriceBumpFlag,
		utils.BlobPoolFetchProbabilityFlag,
		// 同步方式、状态垃圾回收、交易/区块/日志/状态历史保留多久。
		utils.SyncModeFlag,
		utils.SyncTargetFlag,
		utils.ExitWhenSyncedFlag,
		utils.GCModeFlag,
		utils.SnapshotFlag,
		utils.TransactionHistoryFlag,
		utils.ChainHistoryFlag,
		utils.LogHistoryFlag,
		utils.LogNoHistoryFlag,
		utils.LogExportCheckpointsFlag,
		utils.StateHistoryFlag,
		utils.TrienodeHistoryFlag,
		utils.TrienodeHistoryFullValueCheckpointFlag,
		utils.BinTrieGroupDepthFlag,
		utils.LightKDFFlag,
		utils.EthRequiredBlocksFlag,
		// 内存缓存怎么分、文件描述符上限、Go GC，以及 KZG 用哪套实现。
		utils.CacheFlag,
		utils.CacheDatabaseFlag,
		utils.CacheTrieFlag,
		utils.CacheGCFlag,
		utils.CacheSnapshotFlag,
		utils.CacheNoPrefetchFlag,
		utils.CacheNoPrecompileFlag,
		utils.CachePreimagesFlag,
		utils.CacheLogSizeFlag,
		utils.FDLimitFlag,
		utils.MemoryLimitFlag,
		utils.GOGCFlag,
		utils.CryptoKZGFlag,
		// P2P 监听和发现端口、连接上限。
		utils.ListenPortFlag,
		utils.DiscoveryPortFlag,
		utils.MaxPeersFlag,
		utils.MaxPendingPeersFlag,
		// 本地组块参数。合并后真正出块由共识客户端经 engine API 驱动。
		utils.MinerGasLimitFlag,
		utils.MinerGasPriceFlag,
		utils.MinerExtraDataFlag,
		utils.MinerMaxBlobsFlag,
		utils.MinerRecommitIntervalFlag,
		utils.MinerPendingFeeRecipientFlag,
		// NAT、节点发现、允许的网段、本节点密钥。
		utils.NATFlag,
		utils.NoDiscoverFlag,
		utils.DiscoveryV4Flag,
		utils.DiscoveryV5Flag,
		utils.NetrestrictFlag,
		utils.NodeKeyFileFlag,
		utils.NodeKeyHexFlag,
		utils.DNSDiscoveryFlag,
		// --dev：单机开发链，预置账户并自己出块。不给 --datadir 时链在内存里。
		utils.DeveloperFlag,
		utils.DeveloperGasLimitFlag,
		utils.DeveloperPeriodFlag,
		// EVM 调试、内部操作跟踪、执行证明自检。
		utils.VMEnableDebugFlag,
		utils.VMTraceFlag,
		utils.VMTraceJsonConfigFlag,
		utils.VMWitnessStatsFlag,
		utils.VMStatelessSelfValidationFlag,
		// 网络 ID、ethstats 上报、给 eth_gasPrice 用的价格预言机。
		utils.NetworkIdFlag,
		utils.EthStatsURLFlag,
		utils.GpoBlocksFlag,
		utils.GpoPercentileFlag,
		utils.GpoMaxGasPriceFlag,
		utils.GpoIgnoreGasPriceFlag,
		configFileFlag, // --config TOML。命令行上显式写出的 flag 会盖过文件。
		// 进程内信标轻客户端。不配则走 catalyst.Register：JWT 的 engine，外加未认证的 testing。节点自己不出块。
		utils.BeaconApiFlag,
		utils.BeaconApiHeaderFlag,
		utils.BeaconThresholdFlag,
		utils.BeaconNoFilterFlag,
		utils.BeaconConfigFlag,
		utils.BeaconGenesisRootFlag,
		utils.BeaconGenesisTimeFlag,
		utils.BeaconCheckpointFlag,
		utils.BeaconCheckpointFileFlag,
		utils.LogSlowBlockFlag, // 区块执行超过这个时间就打详细日志。
	}, utils.NetworkFlags, utils.DatabaseFlags) // mainnet/sepolia/hoodi，以及 datadir、ancient、状态存储方案。

	rpcFlags = []cli.Flag{
		// HTTP JSON-RPC，默认关闭。
		utils.HTTPEnabledFlag,
		utils.HTTPListenAddrFlag,
		utils.HTTPPortFlag,
		utils.HTTPCORSDomainFlag,
		// 给共识客户端的认证 RPC（engine API），靠 JWT。
		utils.AuthListenFlag,
		utils.AuthPortFlag,
		utils.AuthVirtualHostsFlag,
		utils.JWTSecretFlag,
		utils.HTTPVirtualHostsFlag,
		// GraphQL，同样默认关闭。
		utils.GraphQLEnabledFlag,
		utils.GraphQLCORSDomainFlag,
		utils.GraphQLVirtualHostsFlag,
		utils.HTTPApiFlag,
		utils.HTTPPathPrefixFlag,
		// WebSocket，默认关闭。
		utils.WSEnabledFlag,
		utils.WSListenAddrFlag,
		utils.WSPortFlag,
		utils.WSApiFlag,
		utils.WSAllowedOriginsFlag,
		utils.WSPathPrefixFlag,
		// IPC 默认开启。geth attach 没给端点时拨本机 IPC。geth console 用进程内 Attach。
		utils.IPCDisabledFlag,
		utils.IPCPathFlag,
		// eth_call、发交易、日志查询的资源上限，避免一个请求拖垮节点。
		utils.RPCGlobalGasCapFlag,
		utils.RPCGlobalEVMTimeoutFlag,
		utils.RPCGlobalTxFeeCapFlag,
		utils.RPCGlobalLogQueryLimit,
		utils.EngineMaxReorgDepthFlag, // forkchoiceUpdated 允许把链头回退到规范祖先的最大深度。0 表示不限制。
		utils.AllowUnprotectedTxs,     // 允许没有 EIP-155 链 ID 的交易从 RPC 进来。
		utils.BatchRequestLimit,
		utils.BatchResponseMaxSize,
		utils.HTTPBodyLimitFlag,
		utils.RPCTxSyncDefaultTimeoutFlag,
		utils.RPCTxSyncMaxTimeoutFlag,
		utils.RPCGlobalRangeLimitFlag,
		// RPC 调用的 OpenTelemetry 上报。
		utils.RPCTelemetryFlag,
		utils.RPCTelemetryEndpointFlag,
		utils.RPCTelemetryUserFlag,
		utils.RPCTelemetryPasswordFlag,
		utils.RPCTelemetryInstanceIDFlag,
		utils.RPCTelemetryTagsFlag,
		utils.RPCTelemetrySampleRatioFlag,
	}

	metricsFlags = []cli.Flag{
		// 本机指标 HTTP，以及推到 InfluxDB v1/v2。
		utils.MetricsEnabledFlag,
		utils.MetricsHTTPFlag,
		utils.MetricsPortFlag,
		utils.MetricsEnableInfluxDBFlag,
		utils.MetricsInfluxDBEndpointFlag,
		utils.MetricsInfluxDBDatabaseFlag,
		utils.MetricsInfluxDBUsernameFlag,
		utils.MetricsInfluxDBPasswordFlag,
		utils.MetricsInfluxDBTagsFlag,
		utils.MetricsInfluxDBIntervalFlag,
		utils.MetricsEnableInfluxDBV2Flag,
		utils.MetricsInfluxDBTokenFlag,
		utils.MetricsInfluxDBBucketFlag,
		utils.MetricsInfluxDBOrganizationFlag,
		utils.SnapV2Flag,            // 实验性 snap/2 同步，不适合公网。
		utils.StateSizeTrackingFlag, // 已废弃，仍注册以免旧启动脚本报未知 flag。
	}
)

// app 是 geth 的 urfave/cli 根命令。子命令和 flag 在 init 里挂上。
// 同一套用法的小例子在 golang_demo/urfavecli。
var app = flags.NewApp("the go-ethereum command line interface")

func init() {
	// Initialize the CLI app and start Geth
	//
	// 根命令的 Action。command.go 的 Command.Run 先看第一个位置参数：
	// 命中下面的 Commands 就进那个子命令，否则才调用 geth。
	app.Action = geth
	// 子命令表。app.go 的 newRootCommand 把它挂到根命令的 Subcommands。
	app.Commands = []*cli.Command{
		// See chaincmd.go:
		initCommand,
		importCommand,
		exportCommand,
		importHistoryCommand,
		exportHistoryCommand,
		importPreimagesCommand,
		removedbCommand,
		dumpCommand,
		dumpGenesisCommand,
		pruneHistoryCommand,
		downloadEraCommand,
		// See accountcmd.go:
		accountCommand,
		walletCommand,
		// See consolecmd.go:
		consoleCommand,
		attachCommand,
		javascriptCommand,
		// See misccmd.go:
		versionCommand,
		licenseCommand,
		// See config.go
		dumpConfigCommand,
		// see dbcmd.go
		dbCommand,
		// See cmd/utils/flags_legacy.go
		utils.ShowDeprecated,
		// See snapshot.go
		snapshotCommand,
		// See bintrie_convert.go
		bintrieCommand,
	}
	if logTestCommand != nil {
		app.Commands = append(app.Commands, logTestCommand)
	}
	sort.Sort(cli.CommandsByName(app.Commands))

	app.Flags = slices.Concat(
		nodeFlags,
		rpcFlags,
		consoleFlags,
		debug.Flags,
		metricsFlags,
	)
	flags.AutoEnvVars(app.Flags, "GETH")

	// Command.Run 解析完 flag 之后、进入子命令或 geth 之前跑这段。
	// --help 和 --version 在 checkHelp / checkVersion 里已经返回，不会进到这里。
	// 环境变量前缀是 GETH_。没写在命令行上的 flag 不会在这里盖掉 TOML，覆盖发生在各 setter 的 IsSet。
	app.Before = func(ctx *cli.Context) error {
		maxprocs.Set() // Automatically set GOMAXPROCS to match Linux container CPU quota.
		flags.MigrateGlobalFlags(ctx)
		if err := debug.Setup(ctx); err != nil {
			return err
		}
		flags.CheckEnvVars(ctx, app.Flags, "GETH")
		return nil
	}
	app.After = func(ctx *cli.Context) error {
		debug.Exit()
		prompt.Stdin.Close() // Resets terminal mode.
		return nil
	}
}

// app.Run 把 Action、Before、Commands 收成根命令（app.go newRootCommand），
// 再进 command.go Command.Run：解析 flag，跑 Before，然后按子命令或 geth 分发。
func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// prepare manipulates memory cache allowance and setups metric system.
// This function should be called before launching devp2p stack.
func prepare(ctx *cli.Context) {
	// If we're running a known preset, log it for convenience.
	switch {
	case ctx.Bool(utils.SepoliaFlag.Name):
		log.Info("Starting Geth on Sepolia testnet...")

	case ctx.Bool(utils.HoodiFlag.Name):
		log.Info("Starting Geth on Hoodi testnet...")

	case !ctx.IsSet(utils.NetworkIdFlag.Name):
		log.Info("Starting Geth on Ethereum mainnet...")
	}
}

// geth is the main entry point into the system if no special subcommand is run.
// It creates a default node based on the command line arguments and runs it in
// blocking mode, waiting for it to be shut down.
//
// 走到这里说明第一个位置参数没有命中 app.Commands。
// 多出来的位置参数直接报错，避免把子命令名当成普通参数悄悄忽略。
// stack.Wait 阻塞到 Close 关掉 stop channel。
func geth(ctx *cli.Context) error {
	// 根命令不接受多余的位置参数。能走到这里说明第一个词没命中 Commands（否则 urfave/cli 早就分发到子命令了），例如 `geth foo` 里的 "foo" 就是非法子命令。
	if args := ctx.Args().Slice(); len(args) > 0 {
		return fmt.Errorf("invalid command: %q", args[0])
	}

	prepare(ctx)
	// 根据 flag 组装一个完整的 Ethereum 节点（P2P + 所有协议 + RPC 层），还没 Start，只是把组件挂上去。
	stack := makeFullNode(ctx)
	// 函数返回前关掉 node，释放监听端口和数据库句柄。
	defer stack.Close()

	// 非阻塞地启动 node、RPC/IPC、矿工（false 表示不是 console 模式）。
	startNode(ctx, stack, false)
	// 阻塞当前 goroutine，直到 node 收到退出信号（Ctrl+C 或外部 Stop），
	// 这一行不返回，main 函数也就不退出。
	stack.Wait()
	return nil
}

// startNode boots up the system node and all registered protocols, after which
// it starts the RPC/IPC interfaces and the miner.
//
// 和 utils.StartNode 不是同一个函数。这里只是转调后者，再订钱包插拔事件。
// 控制台模式把 isConsole 传下去。StartNode 丢掉 SIGINT，留给 JS 控制台；SIGTERM 仍然关机。
func startNode(ctx *cli.Context, stack *node.Node, isConsole bool) {
	// Start up the node itself
	utils.StartNode(ctx, stack, isConsole)

	// Register wallet event handlers to open and auto-derive wallets
	events := make(chan accounts.WalletEvent, 16)
	stack.AccountManager().Subscribe(events)

	// Create a client to interact with local geth node.
	rpcClient := stack.Attach()
	ethClient := ethclient.NewClient(rpcClient)

	go func() {
		// Open any wallets already attached
		for _, wallet := range stack.AccountManager().Wallets() {
			if err := wallet.Open(""); err != nil {
				log.Warn("Failed to open wallet", "url", wallet.URL(), "err", err)
			}
		}
		// Listen for wallet event till termination
		for event := range events {
			switch event.Kind {
			case accounts.WalletArrived:
				if err := event.Wallet.Open(""); err != nil {
					log.Warn("New wallet appeared, failed to open", "url", event.Wallet.URL(), "err", err)
				}
			case accounts.WalletOpened:
				status, _ := event.Wallet.Status()
				log.Info("New wallet appeared", "url", event.Wallet.URL(), "status", status)

				var derivationPaths []accounts.DerivationPath
				if event.Wallet.URL().Scheme == "ledger" {
					derivationPaths = append(derivationPaths, accounts.LegacyLedgerBaseDerivationPath)
				}
				derivationPaths = append(derivationPaths, accounts.DefaultBaseDerivationPath)

				event.Wallet.SelfDerive(derivationPaths, ethClient)

			case accounts.WalletDropped:
				log.Info("Old wallet dropped", "url", event.Wallet.URL())
				event.Wallet.Close()
			}
		}
	}()
}
