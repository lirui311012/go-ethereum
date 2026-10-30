# trie：Merkle 状态树

[返回学习总览](../LEARNING.md) · 上一篇 [core/state](../core/state/LEARNING.md) · 下一步 [triedb](../triedb/LEARNING.md)

共识状态根来自树的哈希。`trie.Commit` 只把脏节点收进内存中的 `NodeSet`，磁盘交给 [triedb](../triedb/LEARNING.md)。

## 建议阅读顺序

1. `trie/node.go`：`fullNode`、`shortNode`、`hashNode`、`valueNode`。小于 32 字节的节点内嵌在父节点里。
2. `trie/hasher.go`、`trie/committer.go`、`trie/trie.go`：读、改、哈希、收集脏节点。
3. `trie/secure_trie.go`：`StateTrie`。账户 key 是 `keccak(address)`，存储 key 是 `keccak(slot)`。
4. [trie/bintrie](bintrie/LEARNING.md)：统一二叉树（UBT）。账户、存储、代码块在同一棵树上。
5. [trie/transitiontrie](transitiontrie/LEARNING.md)：迁移期门面。读时 overlay 优先，未命中再读只读 MPT；写只进 overlay。进度在 [core/overlay](../core/overlay/LEARNING.md)。

## MPT

以太坊账户状态是两级树。账户 trie 的叶子是账户（nonce、余额、存储根、代码哈希）。每个合约另有一棵存储 trie，owner 是账户地址哈希（`StorageTrieID`）。

`trie.Commit` 先 `Hash()`，再由 committer 把 dirty 的 `shortNode` / `fullNode` 收进 `NodeSet`，内存里的节点换成 `hashNode`。之后这棵 trie 不能再改。开启 preimage 时，`StateTrie.Commit` 在此之前把原像放进 triedb 的内存表。

## 统一二叉树

`trie/bintrie` 不是 MPT 的另一种编码。`OpenStorageTrie` 在 UBT 上返回同一棵树。`BinaryTrie.Commit` 同样只 `collectNodes`。pathdb 用 `VerklePrefix`（`"v"`）把这套节点和 MPT 数据隔开。

## 和 StateDB 的交界

[StateDB.IntermediateRoot](../core/state/LEARNING.md) 调用存储 trie 的更新和账户 trie 的 `Hash()`。`StateDB.Commit` 再调用 `trie.Commit` 拿到节点集，交给 `triedb.Update`。

## 读的时候记住

- 扁平 snapshot 可以加速读，不参与 `trie.Hash()`。
- 证明和 witness 从 trie 来。
- 改节点编码会改变 state root。生成代码和共识测试要一起看。
