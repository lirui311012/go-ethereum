# core/overlay：MPT 迁到 UBT 的进度

[返回学习总览](../../LEARNING.md) · 树 [trie/bintrie](../../trie/bintrie/LEARNING.md) · 读写门面 [trie/transitiontrie](../../trie/transitiontrie/LEARNING.md)

这个包只有 `TransitionState`：迁移走到哪个账户、哪个槽，以及 MPT 基线根是哪一棵。状态转换循环在 `core`，树的读写在 `transitiontrie`。

## 建议阅读顺序

1. `state_transition.go` 的 `TransitionState`、`InTransition`、`Transitioned`、`LoadTransitionState`。
2. `core/rawdb` 里 `ReadVerkleTransitionState` / `WriteVerkleTransitionState`：进度按状态根存。
3. `core/state/reader.go` 的 `newUBTTrieReader`：用这份进度决定要不要挂上只读 MPT。

## 字段

| 字段 | 含义 |
|------|------|
| `Started` / `Ended` | 迁移是否已经开始、是否已经结束 |
| `CurrentAccountAddress` | 上次翻译完的账户 |
| `CurrentSlotHash` | 该账户上次翻译完的槽哈希 |
| `StorageProcessed` | 这个账户的存储是否已经整段处理完。叶子预算在槽中间用尽时，它仍是 false |
| `CurrentPreimageOffset` | 预映像文件下一次从哪个字节读 |
| `BaseRoot` | 只读 MPT 的根。transitiontrie 用它打开 base |

`InTransition` 是 `Started && !Ended`。`Transitioned` 是 `Ended`。`Copy` 做深拷贝，避免区块处理时改到上一块的进度。

`LoadTransitionState` 用 gob 解出挂在这个状态根上的记录。库里没有记录时新建一份，并把 `Ended` 设成 `isUBT`：从一开始就按 UBT 创建的库视为迁移已经结束，不再去找 MPT。

## 和执行的关系

每个状态根可以有自己的一份进度。重组回到旧根时，读到的是那个根当时写下的进度，而不是进程里的一份全局游标。

翻译循环按账户和槽推进这份结构，写回时跟状态根一起走 `rawdb`。`transitiontrie` 只消费 `InTransition` 和 `BaseRoot`，不推进游标。

## 读的时候记住

- 改进度的编码要同时改 gob 字段和已经落盘的状态根记录。解不出来时 `LoadTransitionState` 打错误日志并返回 nil。只有库里没有记录才会新建，并把 `Ended` 设成 `isUBT`。
- `Ended == true` 有两种来源：迁移跑完，或者这个库创建时就是 UBT。
- 账户或槽的键布局在 `trie/bintrie`。这里只记「迁移到哪了」。
