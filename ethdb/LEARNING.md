# ethdb：键值与冷存储接口

[返回学习总览](../LEARNING.md) · 上一篇 [triedb](../triedb/LEARNING.md) · 下一步 [core/txpool](../core/txpool/LEARNING.md)

`ethdb` 是存储接口，没有链语义。区块 schema 在 `core/rawdb`，trie 节点策略在 [triedb](../triedb/LEARNING.md)。

## 建议阅读顺序

1. `ethdb/database.go`：`KeyValueStore`、`AncientStore`、`Database`（两者合在一起）。
2. `ethdb/batch.go`：`Batch`。`IdealBatchSize` 为 100KiB。`Write()` 才提交。
3. `ethdb/pebble`：链数据默认的 KV 实现。
4. `ethdb/memorydb`：测试和 `--dev` 无 datadir 时的内存库。
5. `ethdb/leveldb`：仍保留的实现。
6. `ethdb/remotedb`：只读的 `debug` 远程读取。

Ancient 的具体文件格式在 `core/rawdb/freezer.go`。chain freezer 约每分钟把远离头部的规范段从 KV 迁到 `ancient/chain/`。读旧块走 ancient，读新块走 KV。

## 谁写什么

| 数据 | 去向 |
|------|------|
| 近期 header、body、receipt | KV，前缀见 `core/rawdb/schema.go` |
| 冷的规范区块 | chain freezer |
| trie 节点 | hashdb 的哈希 key，或 pathdb 的 `A`/`O` |
| 合约字节码 | KV，前缀 `c` |
| path 方案历史 diff | state / trienode freezer |

`check_baddeps` 只拦 `core/rawdb` 依赖 `ethdb/leveldb` 和一条不存在的 `ethdb/pebbledb` 路径。新代码依赖 `ethdb` 接口，在节点打开库的地方选择 Pebble。

## 读的时候记住

- `Put` 进 batch 之后要 `Write`。中途崩溃丢掉的是未写的 batch。
- freezer 只追加。回滚用 `TruncateHead` 丢掉新端，裁历史用 `TruncateTail` 丢掉旧端。
- 换 KV 引擎要同时能打开已有数据目录，或提供迁移。空库默认路径在节点配置里选定。
