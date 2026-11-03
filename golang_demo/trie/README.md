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
       p*  t*
       |
       l
       |
       e*
```

`app` 和 `apple` 共用 `a → p → p`。`apt` 在第二个字符就分叉。查 `ap` 能顺着边走下去，但那个节点没有词尾，所以 `Search("ap")` 是 false，`StartsWith("ap")` 是 true。

## Unicode 码点

ASCII 里一个字符对应一个字节，也对应一个 0 到 127 的编号。`A` 是 65，`a` 是 97。英文字母、数字和常见标点都在这 128 个编号里。

Unicode 给每个字符一个编号，这个编号叫做码点，写成 `U+` 再加十六进制。ASCII 的 128 个字符原样占着 Unicode 最前面的 128 个码点，编号不变：

| 字符 | ASCII | 码点 | 数值 |
|---|---|---|---|
| `A` | 65 | `U+0041` | 65 |
| `a` | 97 | `U+0061` | 97 |
| `你` | 没有这个字符 | `U+4F60` | 20320 |
| `好` | 没有这个字符 | `U+597D` | 22909 |

`你` 的编号是 20320，已经超出 127，所以它没有 ASCII 字节。码点仍是「一个字符一个编号」，只是编号可以比 127 大得多。

码点是字符的编号。UTF-8 是把这个编号写成字节的规则，Go 的 `string` 用的就是它。同一个编号还可以按 UTF-16 或 UTF-32 写成另一串字节，字符仍是同一个。

0 到 127 的码点，UTF-8 规定写成一个字节，值和 ASCII 相同。`A` 在字符串里就是字节 `0x41`。一段纯 ASCII 文本按 UTF-8 来读，字符还是那些字符。超过 127 的编号一个字节省不下，UTF-8 按编号落在哪个区间决定用几个字节。`x` 的位置拼起来就是码点的二进制；以 `10` 开头的字节是前一个字符的延续。

| 码点范围 | 字节数 | 字节形状 |
|---|---|---|
| `U+0000`–`U+007F` | 1 | `0xxxxxxx` |
| `U+0080`–`U+07FF` | 2 | `110xxxxx 10xxxxxx` |
| `U+0800`–`U+FFFF` | 3 | `1110xxxx 10xxxxxx 10xxxxxx` |
| `U+10000`–`U+10FFFF` | 4 | `11110xxx 10xxxxxx 10xxxxxx 10xxxxxx` |

`你` 是 `U+4F60`，落在第三档。`4F60` 的二进制是 `0100 1111 0110 0000`，按 4 位、6 位、6 位切开，再套上第三档的前缀：

```text
0100     111101   100000
 │         │        │
11100100 10111101 10100000
   E4       BD       A0
```

`好` 是 `U+597D`，二进制 `0101 1001 0111 1101`，同样套上第三档的前缀，得到 `E5 A5 BD`。所以：

```text
你  U+4F60  →  E4 BD A0    3 字节
好  U+597D  →  E5 A5 BD    3 字节
你好                   6 字节，2 个码点
```

`len("你好")` 数的是字节，结果是 6。`Insert` 里的 `for _, r := range word` 按码点走，每次交出一个 `rune`（`int32` 的别名），里面放的就是码点的数值：

```go
for _, r := range "你好" {
    // 第一次 r == '你'，值是 0x4F60
    // 第二次 r == '好'，值是 0x597D
}
```

字典树的一条边就是这样一个 `rune`。`你好` 因此是两个节点。按字节来切的话，会走出 6 条边，中间的 `BD`、`A0` 单独看并不是一个字符。

`G` 的码点是 `U+0047`，`g` 是 `U+0067`。`Go` 和 `go` 从第一笔就分成两条路径。

查询表：

- [ASCII 表](https://www.asciitable.com/)：0–127 的十进制、十六进制和字符。
- [Basic Latin（U+0000–U+007F）](https://www.unicode.org/charts/PDF/U0000.pdf)：Unicode 官方码表里和 ASCII 对齐的那一页。
- [Unicode 码表目录](https://www.unicode.org/charts/)：按区块列出全部码表。
- [按字符查码点](https://util.unicode.org/UnicodeJsps/character.jsp)：输入一个字符或 `U+` 编号。[`你` 的 U+4F60](https://www.compart.com/en/unicode/U+4F60) 这一页同时列出 UTF-8 字节 `E4 BD A0`。

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
