# eth/protocols/snap：snap/2 的状态同步

[返回协议](../LEARNING.md) · [返回学习总览](../../../LEARNING.md) · 调度 [eth/downloader](../../downloader/LEARNING.md)

snap/1 的算法在 `sync.go`：按账户哈希区间拉叶子，缺的枝再要 trie 节点。snap/2 在 `syncv2.go`：拉扁平账户、存储和字节码，pivot 移动时用区块访问列表把差额补上，trie 在本地生成。downloader 构造时二选一，`snapV2` 为真才 `NewV2Syncer`，并在 `MakeProtocols` 里宣告 snap/2。

## 建议阅读顺序

1. `protocol.go`：snap/2 比 snap/1 多 `GetAccessLists` / `AccessLists`（`0x08` / `0x09`）。
2. `syncv2.go` 的 `syncPhase`、`syncerV2`、`Sync`。
3. `bal_apply.go`：访问列表如何改扁平状态，并核对头里的 `BlockAccessListHash`。
4. `triedb/generate.go` 的 `GenerateTrieWithProgress`：下载结束后从扁平状态生成 trie。
5. `sync.go` 开头的共享常量，以及 `gentrie.go`：后者是 snap/1 在收叶子时顺手生成节点，snap/2 的下载路径不用它。

## 三个阶段

阶段写在 journal 里的 `syncProgressV2.Phase`，严格按顺序。后一阶段开始时，前面的阶段已经做完。版本字节 `syncProgressVersion` 是 2，对不上就丢掉进度重来。

```mermaid
flowchart LR
    dl["phaseDownload"]
    gen["phaseGenerate"]
    done["phaseComplete"]

    dl -->|"扁平状态齐了"| gen
    gen -->|"本地 trie 对上这个 pivot 根"| done
    done -->|"链头往前"| bal["BAL 追赶，每块提交一次"]
    bal --> done
```

**phaseDownload。** 向 peer 要账户区间、存储区间和字节码，写成扁平状态。请求对着当前 pivot 的状态根。对端只服务还比较新的根，所以这一阶段 pivot 要跟着链头走。`FrozenPivot` 在这一阶段返回 nil。下载过程中不生成 trie，见 `syncv2_runner.go`。

**phaseGenerate。** 下载完成后 `resetTrienodes`，再 `triedb.GenerateTrieWithProgress`，从扁平状态生成对上这个 pivot 根的 trie。`FrozenPivot` 在这一阶段返回当前 pivot，downloader 不能把它挪到别的根。`syncerV2` 自己不发 `RequestTrieNodes`；`OnTrieNodes` 是空操作。

**phaseComplete。** 扁平状态和 trie 都对上了这个 pivot。之后 pivot 可以再动。每一次移动用 BAL 追赶：拉访问列表、核对头里的哈希、把差额应用到扁平状态和 trie，并按块提交。单次窗口是 `catchUpWindow`（512 块）。整段缺口超过 `maxCatchUpBlocks`（`FullImmutabilityThreshold`，90000）就认为对端已经不再保存这些 BAL，丢掉进度重来。单次访问列表请求最多 `maxAccessListRequestCount`（28）个块。

## 前提

`syncerV2` 的注释写明设计建立在 EIP-6780 上：已经下载的状态只向前滚，不再整段重拉。旧的 `SELFDESTRUCT` 会整棵擦掉存储、却不在访问列表里逐槽出现，扁平状态里会留下不该留的槽。因此还开着自毁的网络不能用这条同步。

## 读的时候记住

- 宣告 snap/2 由 `MakeProtocols(backend, snapV2)` 决定。协议常量在 `protocol.go` 里出现，不表示线路上已经在用。
- phaseGenerate 期间 pivot 是冻住的。chain head 在这段时间继续往前，完成后靠 BAL 追，而不是重新下载整段扁平状态。
- 生成出来的 trie 要和 pivot 头的状态根一致。扁平数据对了、生成边界错了，根对不上。
- snap/1 的 heal 阶段要 trie 节点。snap/2 的 `RequestTrieNodes` 留在 peer 接口上，是为了让同一种 peer 也能服务 snap/1。
