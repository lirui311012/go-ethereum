# core/rawdb：链数据的键和 freezer

[返回 core](../LEARNING.md) · [返回学习总览](../../LEARNING.md) · 相关 [ethdb](../../ethdb/LEARNING.md)、[triedb](../../triedb/LEARNING.md)

`rawdb` 规定链数据在键值库里的键，并实现只追加的 freezer。它不知道一笔交易如何执行。键值引擎本身在 [ethdb](../../ethdb/LEARNING.md)。状态树节点的缓存和历史在 [triedb](../../triedb/LEARNING.md)，path 方案用到的键前缀也定义在这里。

## 建议阅读顺序

1. `core/rawdb/schema.go`：前缀和 `headBlockKey` 一类元数据键。
2. `core/rawdb/accessors_chain.go`：`ReadCanonicalHash`、`WriteHeader`、`ReadBody`。
3. `core/rawdb/accessors_indexes.go`：交易查找 `txLookupPrefix`。
4. `core/rawdb/ancient_scheme.go`：freezer 表名。
5. `core/rawdb/chain_freezer.go`：把远离链头的规范段迁到 freezer。
6. `core/rawdb/freezer.go`、`freezer_table.go`：表怎么按高度追加、截断。

## 键值库里的键

高度是大端 `uint64`。常见前缀：

| 前缀 | 键 | 值 |
|--|--|--|
| `h` | 高度 + 哈希 | 块头 |
| `h` + `n` | 高度 | 该高度的规范哈希 |
| `H` | 哈希 | 高度 |
| `b` | 高度 + 哈希 | body |
| `r` | 高度 + 哈希 | 收据 |
| `j` | 高度 + 哈希 | 区块访问列表 |
| `l` | 交易哈希 | 交易在哪一块、哪一笔 |
| `c` | 代码哈希 | 合约字节码 |
| `S` | 高度 | skeleton 同步用的头 |

元数据是完整字符串键，例如 `LastHeader`、`LastBlock`、`LastFast`、`LastFinalized`、`LastPivot`。`GetBlockByNumber` 先读规范哈希（`h` + 高度 + `n`），再按哈希读头和 body。库里可以有侧链块，规范链只认这张「高度 → 哈希」表。

path 方案的 trie 键也在 `schema.go`：账户节点 `A` + hex path，存储节点 `O` + 账户哈希 + hex path，状态编号 `L`。hash 方案的 trie 节点以节点哈希为键，不走这些前缀。读写 trie 的策略在 triedb，不在 chain accessor 里。

快照账户是 `a`，快照存储槽是 `o`。这和 trie 节点的 `A` / `O` 不是同一套数据。

## Freezer

Freezer 按高度编号，只追加。chain freezer 的目录名是 `chain`，表为：

| 表 | 内容 | 能否裁尾 |
|--|--|--|
| `headers` | 块头 | 保留 |
| `hashes` | 规范哈希，不压缩 | 保留 |
| `bodies` | body | 和收据一起裁，组名 `blockdata` |
| `receipts` | 收据 | 同上 |
| `bals` | 区块访问列表 | 单独一组 `bal`，因为 EIP-7928 之后才有 |

`chain_freezer.go` 的冻结线是 `max(已最终确认高度, 链头 - 90000)`，`90000` 是 `params.FullImmutabilityThreshold`。过了这条线的规范段从键值库搬进 freezer，再从热库删掉。创世块留在键值库。交易查找键 `l` 和 `H`（哈希到高度）在冻结后还在。读的时候先看高度是否落在 ancient 范围。回滚用 `truncateHead`。body 和收据共享尾部，避免一边还在、另一边已经删了。

path 方案的历史状态用另一套 state freezer，不在这五张 chain 表里。

## 读的时候记住

- 规范哈希和「库里有这个块」是两回事。侧链块可以有 `h`/`b` 键，但 `h`+高度+`n` 指向另一条。
- 收据、body 的冷数据在 freezer。账户和存储槽在 trie / pathdb。
- `A`/`O` 是 path trie 节点。`a`/`o` 是快照。大小写不同。
- 交易查找键 `l` 可以比区块数据更短。`TransactionIndexTail` 记录索引还留到哪。
