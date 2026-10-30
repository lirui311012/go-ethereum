// kzgdemo 演示 crate-crypto/go-eth-kzg（即以太坊 gokzg 后端本身）的完整用法。
//
// 流程：
//  1. 加载以太坊官方 KZG trusted setup（SRS），构造 Context
//  2. 构造一个合法 Blob（4096 个 < BLS_MODULUS 的 field element，每个 32 字节）
//  3. Blob -> KZG Commitment（48 字节 G1 点压缩形式）
//  4. Blob -> 整体 KZGProof（节点入队 blob 时用它做一致性校验）
//  5. VerifyBlobKZGProof：验证 blob 与 commitment 确实匹配
//  6. 单点 Opening Proof：选一个 field element，证明"这个位置的值是 X"
//  7. VerifyKZGProof：不下载整个 blob，仅凭 commitment + proof 验证单点
//  8. 篡改演示：改一个字节，验证必须失败
package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"

	goethkzg "github.com/crate-crypto/go-eth-kzg"
)

// blsModulusBig 把包内导出的 BlsModulus ([32]byte) 转成 *big.Int，
// 方便我们对随机数取模，保证构造出的每个 field element 都合法。
var blsModulusBig = new(big.Int).SetBytes(goethkzg.BlsModulus[:])

// randomBlob 构造一个随机但合法的 Blob：
// Blob 共 4096 个 field element，每个 32 字节，且必须严格小于 BLS_MODULUS。
func randomBlob() *goethkzg.Blob {
	b := new(goethkzg.Blob)
	var raw [32]byte
	for i := 0; i < goethkzg.ScalarsPerBlob; i++ {
		if _, err := rand.Read(raw[:]); err != nil {
			panic(err)
		}
		v := new(big.Int).SetBytes(raw[:])
		v.Mod(v, blsModulusBig)
		// 取模后写入 32 字节大端
		v.FillBytes(b[i*goethkzg.SerializedScalarSize : (i+1)*goethkzg.SerializedScalarSize])
	}
	return b
}

// fieldElement 读取 blob 中第 i 个 field element（32 字节）。
func fieldElement(blob *goethkzg.Blob, i int) goethkzg.Scalar {
	var s goethkzg.Scalar
	copy(s[:], blob[i*goethkzg.SerializedScalarSize:(i+1)*goethkzg.SerializedScalarSize])
	return s
}

func main() {
	// --- 1. 加载 trusted setup ---
	// NewContext4096Secure 会内嵌以太坊官方的 KZG 可信设置参数（G1/G2 SRS），
	// 无需联网下载；启动时约 2-5 秒解析时间。
	fmt.Println("=== 1. 加载 KZG trusted setup（gokzg 后端）===")
	ctx, err := goethkzg.NewContext4096Secure()
	if err != nil {
		panic(err)
	}

	// --- 2. 构造随机 Blob ---
	fmt.Println("\n=== 2. 构造随机 Blob ===")
	blob := randomBlob()
	// 128KB blob
	fmt.Printf("Blob 长度 = %d 字节 (= %d field elements × %dB)\n",
		len(blob), goethkzg.ScalarsPerBlob, goethkzg.SerializedScalarSize)

	// --- 3. Blob -> KZG Commitment ---
	// numGoRoutines 传 0 表示用默认值；生产环境可设成 runtime.NumCPU() 加速。
	commitment, err := ctx.BlobToKZGCommitment(blob, 0)
	if err != nil {
		panic(err)
	}
	fmt.Println("\n=== 3. KZG Commitment（48B G1 压缩点）===")
	fmt.Println("commitment =", hex.EncodeToString(commitment[:]))

	// --- 4. 计算 blob 整体 KZGProof ---
	// 这个 proof 用来证明"这个 blob 确实对应这个 commitment"。
	// 以太坊节点收到 blob 后会调用 VerifyBlobKZGProof 校验。
	proof, err := ctx.ComputeBlobKZGProof(blob, commitment, 0)
	if err != nil {
		panic(err)
	}
	fmt.Println("\n=== 4. Blob 整体 KZGProof（48B）===")
	fmt.Println("proof =", hex.EncodeToString(proof[:]))

	// --- 5. 验证 blob 整体 proof ---
	fmt.Println("\n=== 5. VerifyBlobKZGProof（节点入队 blob 时调用）===")
	if err := ctx.VerifyBlobKZGProof(blob, commitment, proof); err != nil {
		fmt.Println("❌ 验证失败:", err)
		panic("合法 blob 应验证通过")
	}
	fmt.Println("✅ 验证通过：blob 与 commitment 匹配")

	// --- 6. 单点 Opening Proof：证明多项式在域元素 123 处的值 ---
	// blob 的第 i 格是多项式在第 i 个单位根上的求值，不是 P(i)。
	// 这里的 point 是整数 123 这个域元素，所以证明的是 P(123)，不是第 123 格。
	const pointIndex = 123
	var point goethkzg.Scalar
	// 把整数 123 编码成 32 字节大端。这不是第 123 个单位根。
	new(big.Int).SetInt64(int64(pointIndex)).FillBytes(point[:])

	cell := fieldElement(blob, pointIndex)

	// ComputeKZGProof 返回 (proof, claimedEvaluation)。
	// claimedEvaluation 是 P(point)，和 blob 第 123 格不是同一个数。
	singleProof, claimedEval, err := ctx.ComputeKZGProof(blob, point, 0)
	if err != nil {
		panic(err)
	}
	fmt.Printf("\n=== 6. 单点 Opening Proof（域元素 %d，不是第 %d 格）===\n", pointIndex, pointIndex)
	fmt.Println("cell_123     (第 123 格)   =", hex.EncodeToString(cell[:]))
	fmt.Println("claimed_eval (P(123))      =", hex.EncodeToString(claimedEval[:]))
	fmt.Println("single_proof (48B)          =", hex.EncodeToString(singleProof[:]))

	// --- 7. 不下载整个 blob，仅凭 commitment + proof 验证单点 ---
	fmt.Println("\n=== 7. VerifyKZGProof（轻节点/链上验证）===")
	if err := ctx.VerifyKZGProof(commitment, point, claimedEval, singleProof); err != nil {
		fmt.Println("❌ 验证失败:", err)
		panic("合法 proof 应验证通过")
	}
	fmt.Println("✅ 验证通过：P(123) 确实是", hex.EncodeToString(claimedEval[:]), "，这不是 blob 的第 123 格")

	// --- 8. 篡改演示：改一个 field element 的值（仍保证 < BLS_MODULUS），旧 commitment + 新 proof 必须验不过 ---
	fmt.Println("\n=== 8. 篡改演示：修改最后一个 field element 的值 ===")
	tampered := new(goethkzg.Blob)
	copy(tampered[:], blob[:])
	// 改最后一个 field element：原值 + 1 后对模数取模，保证仍是合法 field element
	lastFEVal := fieldElement(blob, goethkzg.ScalarsPerBlob-1)
	lastFE := new(big.Int).SetBytes(lastFEVal[:])
	lastFE.Add(lastFE, big.NewInt(1))
	lastFE.Mod(lastFE, blsModulusBig)
	lastFE.FillBytes(tampered[(goethkzg.ScalarsPerBlob-1)*goethkzg.SerializedScalarSize:])

	tamperedCommitment, err := ctx.BlobToKZGCommitment(tampered, 0)
	if err != nil {
		panic(err)
	}
	tamperedProof, tamperedClaimedEval, err := ctx.ComputeKZGProof(tampered, point, 0)
	if err != nil {
		panic(err)
	}

	// 攻击场景：攻击者改了 blob 内容，但对外广播的还是旧 commitment
	// 注意：攻击者手里的 proof 是基于新多项式算的，但声称的取值仍是旧多项式在 point 处的值（claimedEval）
	// 此时旧 commitment + 旧 claimedEval + 新 proof 配对，必须验证失败
	attackErr := ctx.VerifyKZGProof(commitment, point, claimedEval, tamperedProof)
	fmt.Println("旧 commitment + 旧 claimedEval + 篡改后 proof 的验证结果:", attackErr)
	if attackErr == nil {
		panic("安全漏洞：篡改后的 proof 竟然通过了！")
	}

	// 反过来：新 commitment + 新 claimedEval + 新 proof 当然能通过（全都是新算的）
	if err := ctx.VerifyKZGProof(tamperedCommitment, point, tamperedClaimedEval, tamperedProof); err != nil {
		fmt.Println("新 commitment + 新 proof 验证失败（意外）:", err)
		panic("新 commitment 应能验证通过")
	} else {
		fmt.Println("新 commitment + 新 claimedEval + 新 proof 验证通过（符合预期，三者都是新算的）")
	}

	// --- 9. 批量验证演示（节点常见用法：一次验证多个 blob）---
	fmt.Println("\n=== 9. 批量验证 VerifyBlobKZGProofBatch（3 个 blob）===")
	var blobs []*goethkzg.Blob
	var commits []goethkzg.KZGCommitment
	var proofs []goethkzg.KZGProof
	for i := 0; i < 3; i++ {
		b := randomBlob()
		c, _ := ctx.BlobToKZGCommitment(b, 0)
		p, _ := ctx.ComputeBlobKZGProof(b, c, 0)
		blobs = append(blobs, b)
		commits = append(commits, c)
		proofs = append(proofs, p)
	}
	if err := ctx.VerifyBlobKZGProofBatch(blobs, commits, proofs); err != nil {
		fmt.Println("❌ 批量验证失败:", err)
		panic("批量验证应通过")
	}
	fmt.Println("✅ 3 个 blob 批量验证通过（一次 pairing 运算搞定）")

	fmt.Println("\n全部流程执行完毕 ✅")
}
