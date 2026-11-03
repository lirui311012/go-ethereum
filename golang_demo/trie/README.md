# 字典树

前缀字典树（Trie）。一个词拆成 Unicode 码点，每个码点是一条边。共享前缀的词共用同一段路径，走到标了词尾的节点才算一个完整的词。

放入 `app`、`apple`、`apt` 之后，树长这样。`*` 表示有词在这个节点结束：

```text
        root
         |
         a
         |
         p
        / \
       p   t*
      / \
     *   l
          |
          e*
```

`app` 和 `apple` 共用 `a → p → p`。`apt` 在第二个字符就分叉。查 `ap` 能顺着边走下去，但那个节点没有词尾，所以 `Search("ap")` 是 false，`StartsWith("ap")` 是 true。

边上的是码点，不是字节。`你好` 是两个节点。`Go` 和 `go` 是两条不同的路径。

零值是一棵空树，可以直接调用方法。`New` 只是返回 `&Trie{}`。

## 方法

| 方法 | 作用 |
|---|---|
| `Insert(word)` | 放入一个词。同一个词再放一次，词数不变 |
| `Search(word)` | 这个字符串是不是一个完整的词 |
| `StartsWith(prefix)` | 是否有词以这个前缀开头 |
| `Delete(word)` | 删掉一个词。词不存在时返回 false，树保持原样 |
| `Len()` | 词的个数。同一个词只计一次 |

空字符串可以作为一个词放进去。空前缀停在根上，空树的 `StartsWith("")` 也是 true。

`Delete` 在返回途中剪掉空分支：一个节点不再是词尾，也没有孩子，就从父节点上摘掉。删掉唯一的 `cat` 之后，`StartsWith("c")` 是 false。`app` 还在时删掉 `apple`，`app` 那条词尾留着，`apple` 多出去的边被剪掉。

## 用法

```go
tr := trie.New()
tr.Insert("apple")
tr.Insert("app")

tr.Search("app")      // true
tr.Search("ap")       // false
tr.StartsWith("ap")   // true
tr.Delete("apple")    // true
tr.Len()              // 1
```

## 测试

在本目录执行：

```sh
go test .
```

带覆盖率：

```sh
go test -cover .
```
