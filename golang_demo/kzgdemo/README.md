# go-eth-kzg 示例（EIP-4844 KZG 多项式承诺）

基于 [`github.com/crate-crypto/go-eth-kzg@v1.5.0`](https://pkg.go.dev/github.com/crate-crypto/go-eth-kzg@v1.5.0) 的完整可运行示例。
这个库就是以太坊客户端日志里 `backend=gokzg` 实际加载的纯 Go KZG 后端实现（无需 CGo、无需 c-kzg）。

---

## 一、先搞懂：KZG 到底是干啥的？

### 1.1 一句话版本

**KZG 是一种"多项式承诺"密码学工具：把一大段数据压缩成一个很短的"指纹"（承诺），之后可以拿出一小段证明，让别人不看原始数据就能相信"这段数据没被改过、某个位置的值确实是我说的那样"。**

### 1.2 为什么需要它？—— 用类比理解

想象你在写一本 1000 页的书，你想让全世界相信：

> "这本书是我写的，而且第 327 页第 5 行写的是'床前明月光'。"

朴素做法有两种，都很笨：

- **做法 A**：把整本书（1000 页）发给所有人。可信，但太贵——每个人都得下载 1000 页。
- **做法 B**：只告诉大家"第 327 页第 5 行是'床前明月光'"。便宜，但没人信——你完全可以瞎编。

**KZG 的做法**是：

1. **出书时**：把整本书算一个 48 字节的"指纹"（commitment，承诺）。把这 48 字节贴出来，全世界都知道"这本书对应这个指纹"。
2. **被问到时**：想象验证者随机挑了一个位置来抽查，比如他问：
   > "嘿，你贴出来的那本书，**第 327 页第 5 行**写的是什么？"

   你**不需要把整本书发过去**，只需要做两件事：
   - 把那一句话本身（32 字节）原文发过去
   - 基于整本书的内容，算一个 48 字节的"证明"（proof）发过去

   这个 proof 的含义是：*"我手里确实有那本书，而且第 327 页第 5 行真的就是这句话。"* 它不是凭空编的——必须基于完整的原书内容才能算出来。
3. **验证者**：只需要拿到 `48B 指纹 + 48B 证明 + 那一句话本身（32B）`，就能在**几毫秒**内确认：
   - 这句话确实来自那本贴了指纹的书（指纹匹配）
   - 这句话没被篡改过（proof 通过密码学校验）

**关键点：验证者自始至终没看过那 1000 页书，只看了 128 字节就完成了验证。**

> 类比一下：这就像你把一本 1000 页的书锁在保险柜里，贴了封条（commitment）。别人想抽查某一页，你不用开锁把书给他，只需要把那一页复印给他，再加一张**保险柜封条没被拆过的鉴定报告**（proof）。他拿着鉴定报告 + 那一页复印件，就能相信这页确实来自那本贴了封条的书。

### 1.2.1 三个新手最容易问的问题

**Q1：那个"那一句话本身（32B）"到底是什么？**

就是 prover 声称的"答案"原文。在我们的例子里，如果你贴出的书第 327 页第 5 行真的写着"床前明月光"，那这 32B 就是这一句的内容（编码成 32 字节）。验证者会拿它和 proof 一起做校验。

**Q2：我随便抄一句"床前明月光"发给验证者，再编一个 proof，他怎么知道我是不是造假？**

问得非常好——这正是 KZG 安全性的核心。答案是：**你根本编不出能通过验证的 proof。**

原因分三层：

1. **proof 不是拍脑袋写的字符串，它必须基于原书算出来。**
   没有原书，你连 proof 都算不出来。具体做法是：拿着原多项式，除以 `(X - 抽查点)`，得到一个商多项式，再把这个商多项式折叠成椭圆曲线点。没有原多项式，这一步就是死的。

2. **proof 和 claimedValue 是密码学绑定的。**
   你不能"先选一句好听的话，再配一个 proof"。KZG 的数学结构保证了：对同一个 commitment（指纹）和同一个抽查点，**只有真正的那句话**才能配出通过验证的 proof。如果你把"床前明月光"改成"疑是地上霜"，哪怕只改一个字，你手里那个 proof 立刻失效，验证会报 `can't verify opening proof`（你跑示例第 8 步就看到了）。

3. **验证者不需要信任你，他自己做一次数学运算就能判断。**
   他拿到 `(commitment, 抽查点, claimedValue, proof)` 后，在椭圆曲线上算一次 pairing（几毫秒）。数学上可以证明：
   - 如果数据没造假 → 一定通过
   - 如果数据造假 → 通过的概率小到可以忽略（约 2⁻²⁵⁶，相当于连中十几次彩票头奖）

   这就是密码学和"信任"的区别：**不是"我相信你没造假"，而是"你数学上就造不出假 proof"。**

> 再用保险柜类比：鉴定报告不是你自己写一张纸条说"封条没拆"，而是**第三方鉴定机构用只有他们才有的仪器**测出来的。你自己在家写一百张"封条没拆"的纸条都没用，因为仪器一读就知道是假的。KZG 里的"仪器"就是椭圆曲线 pairing 运算，谁都能操作，结果客观公正。

**Q3：如果书里别的位置真的写了"白日依山尽"（比如第 5 页第 3 行），而第 327 页第 5 行其实是"床前明月光"。验证者问第 327 页第 5 行，我故意答"白日依山尽"——这句话书里确实有，只是不在被问的位置——能蒙混过关吗？**

这是 KZG 最精妙的地方。答案是：**能发现，而且你连 proof 都算不出来。**

关键在于：proof 不是"我知道书里有某句话"的证明，而是"**在 point 这个具体位置上，多项式的值就是 claimedValue**"的证明。point 和 claimedValue 是**焊死**在一起的。

拆开看：

1. **验证者问的是一个具体的 point**：point 编码成"第 327 页第 5 行"这个位置（数学上是一个域元素 x）。
2. **你手里有原多项式 P**，你当然知道 P(x) 真正的值是"床前明月光"。
3. **你想撒谎说 claimedValue = "白日依山尽"**。但算 proof 的数学要求是：必须把多项式 `P(X) - claimedValue` 除以 `(X - x)`，而且必须**整除**才能得到一个合法的商多项式。
4. **问题来了**：因为 P(x) = "床前明月光" ≠ "白日依山尽"，所以 `P(X) - "白日依山尽"` 在 X = x 处**不为 0**，根本不能被 `(X - x)` 整除。
5. **结果**：你要么算不出商多项式（proof 不存在），要么硬凑一个——但硬凑出来的 proof 塞进验证方程 `e(proof, [s]G2 - x·G2) == e(commitment - claimedValue·G1, G2)` 一定不成立。

> 一句话：**proof 上盖着 point 的钢印。** 你可以把"白日依山尽"复印出来，但那张复印件上印的钢印是"第 5 页第 3 行"。验证者要的是"第 327 页第 5 行"的钢印，你拿错钢印的复印件过去，仪器一扫就知道不对。

**这和考试很像：**
- 老师问："第 3 题答案是什么？"
- 你答了第 7 题的正确答案
- 老师一看题号对不上，直接判错
- 哪怕你答的内容本身是书里真有的、不是瞎编的，**位置错了照样是错**

KZG 比考试严格得多：考试你还能把第 7 题答案抄上去蒙老师，KZG 里你**根本造不出印着"第 3 题"钢印、内容却是第 7 题答案的证明**——钢印是椭圆曲线密码学生成的，改不了。

> 补充：正因为如此，协议里 point 必须由**验证者随机选**（或通过 Fiat-Shamir 从数据哈希出来），不能由 prover 自己选。否则 prover 可以挑一个"自己想撒谎的位置"，把书里别的内容挪过去答。

### 1.3 以太坊为什么需要它？（EIP-4844）

Rollup（Arbitrum、Optimism 等）每天把几十万笔交易打包，要让以太坊 L1 节点"存个底"。如果 L1 真的存全部交易数据，存储成本会爆炸。

EIP-4844 的解法：

- Rollup 把交易数据放进 **Blob**（128KB，相当于那"1000 页书"）。
- 执行交易里带着的是 **32 字节的 versioned hash**。48 字节的 KZG 承诺在 blob sidecar 里，不进账户状态。
- 轻节点不用下载全部 Blob，只验证 KZG proof 就能相信"数据真的在网络里、没被作恶者隐藏或篡改"。

这就是"数据可用性证明（Data Availability Proof）"。

### 1.4 背后的数学直觉（不用懂公式）

KZG 把数据编码成一个**多项式**：
- Blob 有 4096 个"格子"，每个格子 32 字节。它们是多项式在 4096 个单位根上的**求值**，不是 4096 个系数。
- 多项式由这些求值唯一确定。承诺用的是 Lagrange 基，不是单项式系数。

然后：
- **承诺** = 把多项式"折叠"成一个椭圆曲线上的点（48 字节）。这一步是单向的——从指纹推不出原多项式，但改了任何一格求值，指纹就变了。
- **证明** = 我想证明"多项式在点 x 处的取值是 y"，就把多项式除以 `(X - x)` 得到一个商多项式，把这个商多项式也折叠成一个点（48 字节）。
- **验证** = 椭圆曲线上的一次 pairing 运算，数学上保证：只有真正持有原多项式的人，才能造出通过验证的 proof。

> 不需要记住公式，记住三件事就够了：
> 1. **改一个字节，承诺就变**（绑定性）
> 2. **证明只有 48 字节**（常数大小）
> 3. **验证只需一次 pairing**（毫秒级）

---

## 二、本示例在演示什么？

```
Blob (128KB, 4096 个 field element)
   │
   ├───► KZG Commitment (48B, G1 压缩点)
   │         │
   │         └───► VersionedHash (32B, 链上交易实际携带)
   │
   ├───► Blob 整体 KZGProof (48B)  ──► 节点入队 blob 时做一致性校验
   │
   └───► 单点 Opening Proof (48B)  ──► 轻节点/链上不下载 blob 即可验证某个点的值
```

代码分 9 步：

| 步骤 | 干什么 | 对应现实场景 |
|---|---|---|
| 1 | 加载 trusted setup | 客户端启动（你日志里的那行） |
| 2 | 构造随机 Blob | Rollup 准备提交一批交易 |
| 3 | Blob → Commitment | 算出链上要存的 48B 指纹 |
| 4 | 计算 Blob 整体 proof | 给节点做入队校验用 |
| 5 | 验证 Blob 整体 proof | L1 节点收到 blob 后做一致性检查 |
| 6 | 对域元素 123 算 opening proof | 证明的是 P(123)，不是第 123 格 |
| 7 | 验证单点 proof | 轻节点不下载 blob 就相信数据 |
| 8 | 篡改 blob 演示 | 看 KZG 如何拒绝篡改 |
| 9 | 批量验证 3 个 blob | 节点一次 pairing 验多个 blob |

---

## 三、运行

要求 Go 1.23 或更新（见本目录 `go.mod`）。

```bash
cd golang_demo/kzgdemo
go mod tidy
go run .
```

首次运行约 2~3 秒加载内嵌的以太坊官方 trusted setup（SRS）参数，无需联网下载。

### 预期输出

```
=== 1. 加载 KZG trusted setup（gokzg 后端）===
=== 2. 构造随机 Blob ===
Blob 长度 = 131072 字节 (= 4096 field elements × 32B)
=== 3. KZG Commitment（48B G1 压缩点）===
commitment = 8747fcacf1fc30b49...
=== 4. Blob 整体 KZGProof（48B）===
proof = 98a3ba7e61fd938324...
=== 5. VerifyBlobKZGProof（节点入队 blob 时调用）===
✅ 验证通过：blob 与 commitment 匹配
=== 6. 单点 Opening Proof（域元素 123，不是第 123 格）===
...
=== 7. VerifyKZGProof（轻节点/链上验证）===
✅ 验证通过
=== 8. 篡改演示：修改最后一个 field element 的值 ===
旧 commitment + 旧 claimedEval + 篡改后 proof 的验证结果: can't verify opening proof
新 commitment + 新 claimedEval + 新 proof 验证通过（符合预期，三者都是新算的）
=== 9. 批量验证 VerifyBlobKZGProofBatch（3 个 blob）===
✅ 3 个 blob 批量验证通过（一次 pairing 运算搞定）

全部流程执行完毕 ✅
```

---

## 四、API 速查

| 函数 | 作用 |
|---|---|
| `NewContext4096Secure()` | 加载以太坊官方 KZG trusted setup，构造 Context |
| `(*Context).BlobToKZGCommitment(blob, n)` | Blob → 48 字节 KZG Commitment |
| `(*Context).ComputeBlobKZGProof(blob, comm, n)` | 计算 blob 整体 proof（节点入队时用） |
| `(*Context).VerifyBlobKZGProof(blob, comm, proof)` | 校验 blob 与 commitment 是否匹配 |
| `(*Context).ComputeKZGProof(blob, point, n)` | 对多项式在 `point` 处生成 opening proof |
| `(*Context).VerifyKZGProof(comm, point, claimedValue, proof)` | 轻节点验证单点 opening proof |
| `(*Context).VerifyBlobKZGProofBatch(blobs, comms, proofs)` | 批量验证多个 blob（一次 pairing） |

第二个参数 `n` 是 goroutine 数，传 `0` 用默认值；生产环境可设成 `runtime.NumCPU()` 加速。

## 关键常量

| 常量 | 值 | 含义 |
|---|---|---|
| `ScalarsPerBlob` | 4096 | 每个 blob 包含的 field element 数量 |
| `SerializedScalarSize` | 32 | 每个 field element 序列化后字节数 |
| `len(Blob)` | 131072 (128KB) | 一个 blob 的总字节数 |
| `CompressedG1Size` | 48 | commitment / proof 的字节数 |
| `BlsModulus` | 32B | BLS12-381 域的阶，所有 field element 必须严格小于它 |

---

## 五、编码注意事项（踩过的坑）

### 1. field element 必须 `< BLS_MODULUS`
随机字节直接当 field element 大概率超模，`BlobToKZGCommitment` 会报错。正确做法是把随机数对 `goethkzg.BlsModulus` 取模后再写回 blob（示例代码里 `randomBlob()` 就是这么做的）。

### 2. 改一格求值，多项式就变了
KZG 证明绑定的是 `(commitment, point, claimedValue, proof)` 四元组。篡改 blob 后：
- 多项式变了 → 任意点的 `claimedValue` 都变了
- 旧 `commitment` + 新 `proof` + 旧 `claimedValue` 必然验不过
- 只有 `新 commitment + 新 proof + 新 claimedValue`（全部重新算）才能验过

这正是 KZG 的密码学绑定性，也是它能防篡改的根本原因。

### 3. point 是 `[32]byte` 编码的域元素
示例把整数 123 编码成 32 字节当 point。这是域元素 123，不是第 123 个单位根，所以 `claimedEval` 不会等于 blob 的第 123 格。要打开第 i 格，point 必须是那一格的单位根。真实协议里，抽查点由验证者或 Fiat-Shamir 决定，不能由 prover 自己挑。

---

## 六、与 go-ethereum 的关系

- 你在 geth / reth / erigon 日志里看到的 `Initializing the KZG library backend=gokzg`，对应的就是这个库。
- go-ethereum 的 `crypto/kzg4844` 默认用这个库。`--crypto.kzg` 的默认值是 `gokzg`。只有二进制带 `ckzg` 编译标签、并且显式把这个 flag 设成 `ckzg` 时，才改用 C 后端。有 cgo 本身不会切换。
- 本示例直接使用 `go-eth-kzg` 原生 API，绕过封装，便于理解底层。

---

## 七、延伸阅读

- EIP-4844: Shard Blob Transactions — https://eips.ethereum.org/EIPS/eip-4844
- go-eth-kzg 文档 — https://pkg.go.dev/github.com/crate-crypto/go-eth-kzg
- KZG 多项式承诺原理解释（通俗） — https://www.tailcall.dev/blog/kate-commitments/
- KZG 原始论文（Kate, Zaverucha, Goldberg, 2010） — *Constant-Size Commitments to Polynomials and Their Applications*
