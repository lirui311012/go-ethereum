// 用已发布的 github.com/ethereum/go-ethereum 向一个地址转 0.001 ETH。
// 私钥、收款地址和 RPC 从本目录的 .env 读取，不引用当前仓库里的 go-ethereum 源码。
//
//	cp .env.example .env
//	go run .
package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
)

// weiPerMilliEther 是 0.001 ETH，单位 wei。1 ETH = 10^18 wei，所以 0.001 ETH = 10^15 wei。
const weiPerMilliEther = 1_000_000_000_000_000

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "sendeth: %v\n", err)
		os.Exit(1)
	}
}

// run 读配置、连接节点，然后发出一笔转账并打印回执。
func run() error {
	// .env 里的键值写进进程环境变量。文件不存在就直接失败，避免悄悄用到别的环境。
	if err := godotenv.Load(); err != nil {
		return fmt.Errorf("读取 .env: %w", err)
	}

	rpcURL := strings.TrimSpace(os.Getenv("RPC_URL"))
	keyHex := strings.TrimSpace(os.Getenv("PRIVATE_KEY"))
	toHex := strings.TrimSpace(os.Getenv("TO_ADDRESS"))
	if rpcURL == "" || keyHex == "" || toHex == "" {
		return fmt.Errorf("RPC_URL、PRIVATE_KEY、TO_ADDRESS 都必须写在 .env 里")
	}
	// 地址是 0x 加 20 字节（40 个十六进制字符）。这里只检查格式，不检查链上有没有这个账户。
	if !common.IsHexAddress(toHex) {
		return fmt.Errorf("TO_ADDRESS 不是合法地址: %s", toHex)
	}

	// 私钥是 secp256k1 曲线上的 32 字节标量。HexToECDSA 不接受 0x 前缀。
	key, err := crypto.HexToECDSA(strings.TrimPrefix(keyHex, "0x"))
	if err != nil {
		return fmt.Errorf("解析 PRIVATE_KEY: %w", err)
	}

	// 超时盖住整条路径：查询、广播，以及等下一个区块把交易打进收据。
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// JSON-RPC 连接。后面的 chain id、nonce、gas、广播都从这里发出。
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return fmt.Errorf("连接 RPC: %w", err)
	}
	defer client.Close()

	value := big.NewInt(weiPerMilliEther)
	to := common.HexToAddress(toHex)
	tx, receipt, err := send(ctx, client, key, to, value)
	if err != nil {
		return err
	}

	// 账户地址不是私钥的一部分：公钥做 Keccak-256，取最后 20 字节。
	from := crypto.PubkeyToAddress(key.PublicKey)
	fmt.Printf("from:    %s\n", from.Hex())
	fmt.Printf("to:      %s\n", to.Hex())
	fmt.Printf("value:   0.001 ETH\n")
	fmt.Printf("tx:      %s\n", tx.Hash().Hex())
	fmt.Printf("block:   %d\n", receipt.BlockNumber.Uint64())
	fmt.Printf("status:  %d\n", receipt.Status)
	return nil
}

// send 组装一笔原生 ETH 转账，签名后广播，并等到收据出现。
func send(ctx context.Context, client *ethclient.Client, key *ecdsa.PrivateKey, to common.Address, value *big.Int) (*types.Transaction, *types.Receipt, error) {
	from := crypto.PubkeyToAddress(key.PublicKey)

	// chain id 会写进签名（EIP-155）。这笔签名只在这条链上有效，不能拿到另一条链重放。
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("读取 chain id: %w", err)
	}
	fmt.Printf("chainID:  %d\n", chainID.Uint64())

	// nonce 是这个账户下一笔交易的序号，从 0 连续递增。
	// pending 把还在交易池里、尚未上链的交易也算进去，避免和自己刚发出的交易撞号。
	nonce, err := client.PendingNonceAt(ctx, from)
	if err != nil {
		return nil, nil, fmt.Errorf("读取 nonce: %w", err)
	}
	fmt.Printf("nonce:  %d\n", nonce)

	// nil 表示最新区块。BaseFee 非空说明这条链已经按 EIP-1559 计费。
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("读取最新区块: %w", err)
	}

	// 转给普通账户固定是 21000 gas。收款方若是合约，执行收款代码会花更多，所以让节点按当前状态估算。
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  from,
		To:    &to,
		Value: value,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("估算 gas: %w", err)
	}
	fmt.Printf("gasLimit:  %d\n", gasLimit)

	tx, err := newTransfer(ctx, client, chainID, nonce, gasLimit, to, value, header)
	if err != nil {
		return nil, nil, err
	}

	if err := canPay(ctx, client, from, value, tx); err != nil {
		return nil, nil, err
	}

	// 第二个参数是 Signer。它不持有私钥，只规定两件事：交易的哪些字段做成待签名哈希，以及签名怎么写进 V/R/S。
	// LatestSignerForChainID 在只知道 chain id、不知道分叉高度时，返回这份库支持的最宽规则（这里是 Prague）。
	// 它覆盖旧式交易和 EIP-1559 等已实现的类型，并把 chain id 算进哈希，所以这笔签名不能拿到另一条链重放。
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), key)
	if err != nil {
		return nil, nil, fmt.Errorf("签名: %w", err)
	}
	// eth_sendRawTransaction：节点验签名、查 nonce 和余额，通过后放进交易池。
	if err := client.SendTransaction(ctx, signed); err != nil {
		return nil, nil, fmt.Errorf("广播交易: %w", err)
	}

	// 轮询收据。收据出现表示已经被打包；Status 为 1 才是执行成功，0 是上链但回滚。
	receipt, err := bind.WaitMined(ctx, client, signed)
	if err != nil {
		return signed, nil, fmt.Errorf("等待上链 %s: %w", signed.Hash().Hex(), err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return signed, receipt, fmt.Errorf("交易已上链但执行失败: %s", signed.Hash().Hex())
	}
	return signed, receipt, nil
}

// newTransfer 构造还没签名的转账。有 base fee 用 EIP-1559，否则退回旧的单一 gasPrice。
func newTransfer(ctx context.Context, client *ethclient.Client, chainID *big.Int, nonce, gasLimit uint64, to common.Address, value *big.Int, header *types.Header) (*types.Transaction, error) {
	if header.BaseFee == nil {
		// 伦敦升级前整笔交易只有一个 gasPrice，出价低于节点建议价就很难被打包。
		gasPrice, err := client.SuggestGasPrice(ctx)
		if err != nil {
			return nil, fmt.Errorf("读取 gas price: %w", err)
		}
		return types.NewTx(&types.LegacyTx{
			Nonce:    nonce,
			GasPrice: gasPrice,
			Gas:      gasLimit,
			To:       &to,
			Value:    value,
		}), nil
	}

	// tip 是 maxPriorityFeePerGas，付给区块的手续费接收地址。base fee 则被烧掉。
	tip, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 priority fee: %w", err)
	}
	fmt.Printf("tip:  %d\n", tip.Uint64())
	// maxFeePerGas = 2 × baseFee + tip。
	// 下一档区块的 baseFee 最多涨 12.5%，留一倍余量，避免排队时 baseFee 上涨把交易挤掉。
	// tip 是给验证者的小费，不随 baseFee 翻倍。
	feeCap := new(big.Int).Add(new(big.Int).Mul(header.BaseFee, big.NewInt(2)), tip)
	fmt.Printf("feeCap:  %d\n", feeCap.Uint64())
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: tip,
		GasFeeCap: feeCap,
		Gas:       gasLimit,
		To:        &to,
		Value:     value,
	}), nil
}

// canPay 在签名和广播之前检查余额。节点也会查，这里提前失败，避免签完才被拒绝。
func canPay(ctx context.Context, client *ethclient.Client, from common.Address, value *big.Int, tx *types.Transaction) error {
	// 最新已确认余额，不含还在交易池里、尚未打包的支出。
	balance, err := client.BalanceAt(ctx, from, nil)
	if err != nil {
		return fmt.Errorf("读取余额: %w", err)
	}
	// 动态费用交易用每 gas 费用上限。旧式交易没有这个字段，退回用 GasPrice。
	feeCap := tx.GasFeeCap()
	if feeCap == nil || feeCap.Sign() == 0 {
		feeCap = tx.GasPrice()
	}
	// 发送前余额至少要覆盖：转账金额 + gas 上限 × 每 gas 费用上限。
	// 实际扣款通常更少：没用完的 gas 会退回，EIP-1559 也只烧掉 baseFee，多出来的 feeCap 会退回。
	need := new(big.Int).Add(value, new(big.Int).Mul(new(big.Int).SetUint64(tx.Gas()), feeCap))
	if balance.Cmp(need) < 0 {
		return fmt.Errorf("余额不足: 有 %s wei，至少需要 %s wei（含 gas 上限）", balance, need)
	}
	return nil
}
