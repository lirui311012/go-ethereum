// Copyright 2017 The go-ethereum Authors
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

package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"unicode"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/external"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/accounts/scwallet"
	"github.com/ethereum/go-ethereum/accounts/usbwallet"
	"github.com/ethereum/go-ethereum/beacon/blsync"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/eth/syncer"
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/ethereum/go-ethereum/internal/telemetry/tracesetup"
	"github.com/ethereum/go-ethereum/internal/version"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/naoina/toml"
	"github.com/urfave/cli/v2"
)

var (
	dumpConfigCommand = &cli.Command{
		Action:      dumpConfig,
		Name:        "dumpconfig",
		Usage:       "Export configuration values in a TOML format",
		ArgsUsage:   "<dumpfile (optional)>",
		Flags:       slices.Concat(nodeFlags, rpcFlags),
		Description: `Export configuration values in TOML format (to stdout by default).`,
	}

	configFileFlag = &cli.StringFlag{
		Name:     "config",
		Usage:    "TOML configuration file",
		Category: flags.EthCategory,
	}
)

// These settings ensure that TOML keys use the same names as Go struct fields.
var tomlSettings = toml.Config{
	NormFieldName: func(rt reflect.Type, key string) string {
		return key
	},
	FieldToKey: func(rt reflect.Type, field string) string {
		return field
	},
	MissingField: func(rt reflect.Type, field string) error {
		id := fmt.Sprintf("%s.%s", rt.String(), field)
		if deprecatedConfigFields[id] {
			log.Warn(fmt.Sprintf("Config field '%s' is deprecated and won't have any effect.", id))
			return nil
		}
		var link string
		if unicode.IsUpper(rune(rt.Name()[0])) && rt.PkgPath() != "main" {
			link = fmt.Sprintf(", see https://godoc.org/%s#%s for available fields", rt.PkgPath(), rt.Name())
		}
		return fmt.Errorf("field '%s' is not defined in %s%s", field, rt.String(), link)
	},
}

var deprecatedConfigFields = map[string]bool{
	"ethconfig.Config.EVMInterpreter":          true,
	"ethconfig.Config.EWASMInterpreter":        true,
	"ethconfig.Config.TrieCleanCacheJournal":   true,
	"ethconfig.Config.TrieCleanCacheRejournal": true,
	"ethconfig.Config.LightServ":               true,
	"ethconfig.Config.LightIngress":            true,
	"ethconfig.Config.LightEgress":             true,
	"ethconfig.Config.LightPeers":              true,
	"ethconfig.Config.LightNoPrune":            true,
	"ethconfig.Config.LightNoSyncServe":        true,
	"ethconfig.Config.EnableStateSizeTracking": true,
}

type ethstatsConfig struct {
	URL string `toml:",omitempty"`
}

// gethConfig 是进程这一份配置的根。TOML 的键名与字段名一致，不转成小写。
type gethConfig struct {
	Eth      ethconfig.Config
	Node     node.Config
	Ethstats ethstatsConfig
	Metrics  metrics.Config
}

func loadConfig(file string, cfg *gethConfig) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	err = tomlSettings.NewDecoder(bufio.NewReader(f)).Decode(cfg)
	// Add file name to errors that have a line number.
	if _, ok := err.(*toml.LineError); ok {
		err = errors.New(file + ", " + err.Error())
	}
	return err
}

func defaultNodeConfig() node.Config {
	git, _ := version.VCS()
	cfg := node.DefaultConfig
	cfg.Name = clientIdentifier
	cfg.Version = version.WithCommit(git.Commit, git.Date)
	cfg.HTTPModules = append(cfg.HTTPModules, "eth")
	cfg.WSModules = append(cfg.WSModules, "eth")
	cfg.IPCPath = clientIdentifier + ".ipc"
	return cfg
}

// loadBaseConfig loads the gethConfig based on the given command line
// parameters and config file.
//
// 顺序是代码默认值，然后 --config 的 TOML，然后显式写在命令行上的节点 flag。
// 协议侧的 flag 要等账户后端装好，在 makeConfigNode 里才写入。
func loadBaseConfig(ctx *cli.Context) gethConfig {
	// Load defaults.
	cfg := gethConfig{
		Eth:     ethconfig.Defaults,
		Node:    defaultNodeConfig(),
		Metrics: metrics.DefaultConfig,
	}

	// Load config file.
	if file := ctx.String(configFileFlag.Name); file != "" {
		if err := loadConfig(file, &cfg); err != nil {
			utils.Fatalf("%v", err)
		}
	}

	// Apply flags.
	utils.SetNodeConfig(ctx, &cfg.Node)
	return cfg
}

// makeConfigNode loads geth configuration and creates a blank node instance.
//
// node.New 会锁住数据目录。账户后端必须先装上，再 SetEthConfig：--dev 要在这里解锁开发账户。
func makeConfigNode(ctx *cli.Context) (*node.Node, gethConfig) {
	cfg := loadBaseConfig(ctx)
	stack, err := node.New(&cfg.Node)
	if err != nil {
		utils.Fatalf("Failed to create the protocol stack: %v", err)
	}
	// Node doesn't by default populate account manager backends
	if err := setAccountManagerBackends(stack.Config(), stack.AccountManager(), stack.KeyStoreDir()); err != nil {
		utils.Fatalf("Failed to set account manager backends: %v", err)
	}

	utils.SetEthConfig(ctx, stack, &cfg.Eth)
	if ctx.IsSet(utils.EthStatsURLFlag.Name) {
		cfg.Ethstats.URL = ctx.String(utils.EthStatsURLFlag.Name)
	}
	applyMetricConfig(ctx, &cfg)

	return stack, cfg
}

// constructs the disclaimer text block which will be printed in the logs upon
// startup when Geth is running in dev mode.
func constructDevModeBanner(ctx *cli.Context, cfg gethConfig) string {
	devModeBanner := `You are running Geth in --dev mode. Please note the following:

  1. This mode is only intended for fast, iterative development without assumptions on
     security or persistence.
  2. The database is created in memory unless specified otherwise. Therefore, shutting down
     your computer or losing power will wipe your entire block data and chain state for
     your dev environment.
  3. A random, pre-allocated developer account will be available and unlocked as
     eth.coinbase, which can be used for testing. The random dev account is temporary,
     stored on a ramdisk, and will be lost if your machine is restarted.
  4. Mining is enabled by default. However, the client will only seal blocks if transactions
     are pending in the mempool. The miner's minimum accepted gas price is 1.
  5. Networking is disabled; there is no listen-address, the maximum number of peers is set
     to 0, and discovery is disabled.
`
	if !ctx.IsSet(utils.DataDirFlag.Name) {
		devModeBanner += fmt.Sprintf(`

 Running in ephemeral mode.  The following account has been prefunded in the genesis:

       Account
       ------------------
       0x%x (10^49 ETH)
`, cfg.Eth.Miner.PendingFeeRecipient)
		if cfg.Eth.Miner.PendingFeeRecipient == utils.DeveloperAddr {
			devModeBanner += fmt.Sprintf(`
       Private Key
       ------------------
       0x%x
`, crypto.FromECDSA(utils.DeveloperKey))
		}
	}

	return devModeBanner
}

// makeFullNode loads geth configuration and creates the Ethereum backend.
func makeFullNode(ctx *cli.Context) *node.Node {
	stack, cfg := makeConfigNode(ctx)
	if ctx.IsSet(utils.OverrideOsaka.Name) {
		v := ctx.Uint64(utils.OverrideOsaka.Name)
		cfg.Eth.OverrideOsaka = &v
	}
	if ctx.IsSet(utils.OverrideAmsterdam.Name) {
		v := ctx.Uint64(utils.OverrideAmsterdam.Name)
		cfg.Eth.OverrideAmsterdam = &v
	}
	if ctx.IsSet(utils.OverrideBPO1.Name) {
		v := ctx.Uint64(utils.OverrideBPO1.Name)
		cfg.Eth.OverrideBPO1 = &v
	}
	if ctx.IsSet(utils.OverrideBPO2.Name) {
		v := ctx.Uint64(utils.OverrideBPO2.Name)
		cfg.Eth.OverrideBPO2 = &v
	}
	if ctx.IsSet(utils.OverrideUBT.Name) {
		v := ctx.Uint64(utils.OverrideUBT.Name)
		cfg.Eth.OverrideUBT = &v
	}

	// Start metrics export if enabled.
	utils.SetupMetrics(&cfg.Metrics)

	// Setup OpenTelemetry reporting if enabled.
	if err := tracesetup.SetupTelemetry(cfg.Node.OpenTelemetry, stack); err != nil {
		utils.Fatalf("failed to setup OpenTelemetry: %v", err)
	}

	// 在这里调用 eth.New：数据库、区块链、交易池都建好，并注册成协议、API 和 Lifecycle。
	// 此时尚未 Start，发现和同步循环还没跑。
	backend, eth := utils.RegisterEthService(stack, &cfg.Eth)

	// Create gauge with geth system and build information
	if eth != nil { // The 'eth' backend may be nil in light mode
		var protos []string
		for _, p := range eth.Protocols() {
			protos = append(protos, fmt.Sprintf("%v/%d", p.Name, p.Version))
		}
		metrics.NewRegisteredGaugeInfo("geth/info", nil).Update(metrics.GaugeInfoValue{
			"arch":      runtime.GOARCH,
			"os":        runtime.GOOS,
			"version":   cfg.Node.Version,
			"protocols": strings.Join(protos, ","),
		})
	}

	// Configure log filter RPC API.
	filterSystem := utils.RegisterFilterAPI(stack, backend, &cfg.Eth)

	// Configure GraphQL if requested.
	if ctx.Bool(utils.GraphQLEnabledFlag.Name) {
		utils.RegisterGraphQLService(stack, backend, filterSystem, &cfg.Node)
	}
	// Add the Ethereum Stats daemon if requested.
	if cfg.Ethstats.URL != "" {
		utils.RegisterEthStatsService(stack, backend, cfg.Ethstats.URL)
	}

	// Configure synchronization override service
	syncConfig := syncer.Config{
		ExitWhenSynced: ctx.Bool(utils.ExitWhenSyncedFlag.Name),
	}
	if ctx.IsSet(utils.SyncTargetFlag.Name) {
		target := ctx.String(utils.SyncTargetFlag.Name)
		if !common.IsHexHash(target) {
			utils.Fatalf("sync target hash is not a valid hex hash: %s", target)
		}
		syncConfig.TargetBlock = common.HexToHash(target)
	}
	utils.RegisterSyncOverrideService(stack, eth, syncConfig)

	// 共识三选一，且都必须发生在 stack.Start 之前。
	// --dev：进程内模拟信标，自己按周期出块。
	// --beacon.api：blsync 轻客户端，经进程内 engine API 把信标头送进来。
	// 否则 catalyst.Register 挂上要 JWT 的 engine，并额外注册未认证的 testing。
	// 这条路径不注册 Lifecycle，节点不会自己按槽出块，等外部共识客户端调用。
	if ctx.Bool(utils.DeveloperFlag.Name) {
		// Start dev mode.
		simBeacon, err := catalyst.NewSimulatedBeacon(ctx.Uint64(utils.DeveloperPeriodFlag.Name), cfg.Eth.Miner.PendingFeeRecipient, eth)
		if err != nil {
			utils.Fatalf("failed to register dev mode catalyst service: %v", err)
		}
		catalyst.RegisterSimulatedBeaconAPIs(stack, simBeacon)
		stack.RegisterLifecycle(simBeacon)

		banner := constructDevModeBanner(ctx, cfg)
		for _, line := range strings.Split(banner, "\n") {
			log.Warn(line)
		}
	} else if ctx.IsSet(utils.BeaconApiFlag.Name) {
		// Start blsync mode.
		srv := rpc.NewServer()
		srv.RegisterName("engine", catalyst.NewConsensusAPI(eth))
		blsyncer := blsync.NewClient(utils.MakeBeaconLightConfig(ctx))
		blsyncer.SetEngineRPC(rpc.DialInProc(srv))
		stack.RegisterLifecycle(blsyncer)
	} else {
		// Launch the engine API for interacting with external consensus client.
		err := catalyst.Register(stack, eth)
		if err != nil {
			utils.Fatalf("failed to register catalyst service: %v", err)
		}
	}
	return stack
}

// dumpConfig is the dumpconfig command.
func dumpConfig(ctx *cli.Context) error {
	_, cfg := makeConfigNode(ctx)
	comment := ""

	// 只是不把创世块写进这份 TOML。磁盘上的创世数据不动。
	// makeConfigNode 已经 node.New，所以本命令会占用数据目录锁，但不启动网络。
	if cfg.Eth.Genesis != nil {
		cfg.Eth.Genesis = nil
		comment += "# Note: this config doesn't contain the genesis block.\n\n"
	}

	out, err := tomlSettings.Marshal(&cfg)
	if err != nil {
		return err
	}

	dump := os.Stdout
	if ctx.NArg() > 0 {
		dump, err = os.OpenFile(ctx.Args().Get(0), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer dump.Close()
	}
	dump.WriteString(comment)
	dump.Write(out)

	return nil
}

func applyMetricConfig(ctx *cli.Context, cfg *gethConfig) {
	if ctx.IsSet(utils.MetricsEnabledFlag.Name) {
		cfg.Metrics.Enabled = ctx.Bool(utils.MetricsEnabledFlag.Name)
	}
	if ctx.IsSet(utils.MetricsHTTPFlag.Name) {
		cfg.Metrics.HTTP = ctx.String(utils.MetricsHTTPFlag.Name)
	}
	if ctx.IsSet(utils.MetricsPortFlag.Name) {
		cfg.Metrics.Port = ctx.Int(utils.MetricsPortFlag.Name)
	}
	if ctx.IsSet(utils.MetricsEnableInfluxDBFlag.Name) {
		cfg.Metrics.EnableInfluxDB = ctx.Bool(utils.MetricsEnableInfluxDBFlag.Name)
	}
	if ctx.IsSet(utils.MetricsInfluxDBEndpointFlag.Name) {
		cfg.Metrics.InfluxDBEndpoint = ctx.String(utils.MetricsInfluxDBEndpointFlag.Name)
	}
	if ctx.IsSet(utils.MetricsInfluxDBDatabaseFlag.Name) {
		cfg.Metrics.InfluxDBDatabase = ctx.String(utils.MetricsInfluxDBDatabaseFlag.Name)
	}
	if ctx.IsSet(utils.MetricsInfluxDBUsernameFlag.Name) {
		cfg.Metrics.InfluxDBUsername = ctx.String(utils.MetricsInfluxDBUsernameFlag.Name)
	}
	if ctx.IsSet(utils.MetricsInfluxDBPasswordFlag.Name) {
		cfg.Metrics.InfluxDBPassword = ctx.String(utils.MetricsInfluxDBPasswordFlag.Name)
	}
	if ctx.IsSet(utils.MetricsInfluxDBTagsFlag.Name) {
		cfg.Metrics.InfluxDBTags = ctx.String(utils.MetricsInfluxDBTagsFlag.Name)
	}
	if ctx.IsSet(utils.MetricsInfluxDBIntervalFlag.Name) {
		cfg.Metrics.InfluxDBInterval = ctx.Duration(utils.MetricsInfluxDBIntervalFlag.Name)
	}
	if ctx.IsSet(utils.MetricsEnableInfluxDBV2Flag.Name) {
		cfg.Metrics.EnableInfluxDBV2 = ctx.Bool(utils.MetricsEnableInfluxDBV2Flag.Name)
	}
	if ctx.IsSet(utils.MetricsInfluxDBTokenFlag.Name) {
		cfg.Metrics.InfluxDBToken = ctx.String(utils.MetricsInfluxDBTokenFlag.Name)
	}
	if ctx.IsSet(utils.MetricsInfluxDBBucketFlag.Name) {
		cfg.Metrics.InfluxDBBucket = ctx.String(utils.MetricsInfluxDBBucketFlag.Name)
	}
	if ctx.IsSet(utils.MetricsInfluxDBOrganizationFlag.Name) {
		cfg.Metrics.InfluxDBOrganization = ctx.String(utils.MetricsInfluxDBOrganizationFlag.Name)
	}
	// Sanity-check the commandline flags. It is fine if some unused fields is part
	// of the toml-config, but we expect the commandline to only contain relevant
	// arguments, otherwise it indicates an error.
	var (
		enableExport   = ctx.Bool(utils.MetricsEnableInfluxDBFlag.Name)
		enableExportV2 = ctx.Bool(utils.MetricsEnableInfluxDBV2Flag.Name)
	)
	if enableExport || enableExportV2 {
		v1FlagIsSet := ctx.IsSet(utils.MetricsInfluxDBUsernameFlag.Name) ||
			ctx.IsSet(utils.MetricsInfluxDBPasswordFlag.Name)

		v2FlagIsSet := ctx.IsSet(utils.MetricsInfluxDBTokenFlag.Name) ||
			ctx.IsSet(utils.MetricsInfluxDBOrganizationFlag.Name) ||
			ctx.IsSet(utils.MetricsInfluxDBBucketFlag.Name)

		if enableExport && v2FlagIsSet {
			utils.Fatalf("Flags --%s, --%s, --%s are only available for influxdb-v2", utils.MetricsInfluxDBOrganizationFlag.Name, utils.MetricsInfluxDBTokenFlag.Name, utils.MetricsInfluxDBBucketFlag.Name)
		} else if enableExportV2 && v1FlagIsSet {
			utils.Fatalf("Flags --%s, --%s are only available for influxdb-v1", utils.MetricsInfluxDBUsernameFlag.Name, utils.MetricsInfluxDBPasswordFlag.Name)
		}
	}
}

func setAccountManagerBackends(conf *node.Config, am *accounts.Manager, keydir string) error {
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	if conf.UseLightweightKDF {
		scryptN = keystore.LightScryptN
		scryptP = keystore.LightScryptP
	}

	// Assemble the supported backends
	if len(conf.ExternalSigner) > 0 {
		log.Info("Using external signer", "url", conf.ExternalSigner)
		if extBackend, err := external.NewExternalBackend(conf.ExternalSigner); err == nil {
			am.AddBackend(extBackend)
			return nil
		} else {
			return fmt.Errorf("error connecting to external signer: %v", err)
		}
	}

	// 外部签名器和本地 keystore 二选一。同时挂上会让同一账户出现两次。
	// For now, we're using EITHER external signer OR local signers.
	// If/when we implement some form of lockfile for USB and keystore wallets,
	// we can have both, but it's very confusing for the user to see the same
	// accounts in both externally and locally, plus very racey.
	am.AddBackend(keystore.NewKeyStore(keydir, scryptN, scryptP))
	if conf.USB {
		// Start a USB hub for Ledger hardware wallets
		if ledgerhub, err := usbwallet.NewLedgerHub(); err != nil {
			log.Warn(fmt.Sprintf("Failed to start Ledger hub, disabling: %v", err))
		} else {
			am.AddBackend(ledgerhub)
		}
		// Start a USB hub for Trezor hardware wallets (HID version)
		if trezorhub, err := usbwallet.NewTrezorHubWithHID(); err != nil {
			log.Warn(fmt.Sprintf("Failed to start HID Trezor hub, disabling: %v", err))
		} else {
			am.AddBackend(trezorhub)
		}
		// Start a USB hub for Trezor hardware wallets (WebUSB version)
		if trezorhub, err := usbwallet.NewTrezorHubWithWebUSB(); err != nil {
			log.Warn(fmt.Sprintf("Failed to start WebUSB Trezor hub, disabling: %v", err))
		} else {
			am.AddBackend(trezorhub)
		}
	}
	if len(conf.SmartCardDaemonPath) > 0 {
		// Start a smart card hub
		if schub, err := scwallet.NewHub(conf.SmartCardDaemonPath, scwallet.Scheme, keydir); err != nil {
			log.Warn(fmt.Sprintf("Failed to start smart card hub, disabling: %v", err))
		} else {
			am.AddBackend(schub)
		}
	}

	return nil
}
