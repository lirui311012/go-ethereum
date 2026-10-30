# tests：状态测试与区块测试

[返回学习总览](../LEARNING.md) · 相关 [core](../core/LEARNING.md)、[core/vm](../core/vm/LEARNING.md)

`tests` 把以太坊共识测试向量跑在 geth 的执行路径上。改 EVM、gas 或区块校验时，用这里确认结果和测试向量一致。它不参与节点启动。

## 建议阅读顺序

1. `tests/init_test.go`：`baseDir`、`stateTestDir`、`blockTestDir`、`benchmarksDir`。
2. `tests/init.go` 的 `Forks`：每个分叉名对应一份 `ChainConfig`。
3. `tests/state_test.go` 的 `TestState`、`execStateTest`。
4. `tests/state_test_util.go` 的 `StateTest.Run`。
5. `tests/block_test.go` 的 `TestBlockchain`，以及 `block_test_util.go` 的 `BlockTest.Run`。
6. `tests/transaction_test.go`、`rlp_test.go`、`difficulty_test.go`：编码和旧难度测试。

## 向量放在哪

两个目录是 git 子模块，默认的源码树里是空的：

| 路径 | 内容 |
|--|--|
| `tests/testdata` | `ethereum/tests`。`GeneralStateTests`、`BlockchainTests`、`TransactionTests` 等 |
| `tests/evm-benchmarks` | `ipsilon/evm-benchmarks`。`TestState` 也会走进去 |

`init_test.go` 把路径拼出来。目录不存在时 `walk` 会 `t.Skip("missing test files")`，这组测试被跳过。日常改一个小包，不必先克隆它们。

执行规范测试是另一套，不在子模块里。`TestExecutionSpecState`、`TestExecutionSpecBlocktests`、`TestExecutionSpecTransaction` 读 `tests/spec-tests/fixtures`。目录不存在就跳过。`go run ./build/ci.go test` 在不带 `-short` 时会下载这份向量；`-short` 不下载。

## 状态测试怎么跑

`TestState` 遍历三处：`testdata/EIPTests/StateTests`、`GeneralStateTests`、`evm-benchmarks/benchmarks`。每个 JSON 解成 `StateTest`。

`execStateTest` 对 `Subtests()` 里的每个分叉调用 `StateTest.Run`。`Run` 用 `genesis` 建初始状态，再执行交易，只比对状态根和日志哈希，不比对 gas。分叉名必须在 `Forks` 里。

失败期望写在向量里。`checkError` 只看有没有错误：向量要求失败时，任何非空错误都算通过，不比较异常种类。

## 区块测试怎么跑

`TestBlockchain` 走 `BlockchainTests`。`BlockTest.Run` 建 `core.BlockChain`，`insertBlocks` 逐块插入，再 `validatePostState` 和 `validateImportedHeaders`。这条路径经过 `InsertChain`。

`testMatcher` 用正则把特别慢或已知不适用的文件标成 `slow` 或 `skipLoad`。看到某个向量没跑，先查 `state_test.go` 和 `block_test.go` 顶部的这些规则。

## 怎么执行

在仓库根目录，子模块已经初始化时：

```sh
go test ./tests -run TestState -count=1
go test ./tests -run TestBlockchain -count=1
```

短迭代用 `go test ./tests -short`。提交前的全量入口是 `go run ./build/ci.go test`，它会带上这组测试。只改 `core/vm` 的一个操作码时，先跑对应包的单测，再按需要加 `TestState`。

## 读的时候记住

- 通用状态测试和区块测试的向量在两个子模块里。执行规范测试在 `tests/spec-tests`，由不带 `-short` 的 `build/ci.go test` 下载。
- `Forks` 决定 `Paris`、`Cancun` 这些名字变成哪份 `ChainConfig`。新分叉要在这里登记，状态测试才会覆盖它。
- 状态测试走 `StateTest.Run` → `ApplyMessage`，不建 `BlockChain`。区块测试走 `BlockTest.Run` → `InsertChain`。
- `skipLoad` 是有意跳过。新增跳过规则时要在旁边写原因。
