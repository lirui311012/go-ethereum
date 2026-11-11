# os.Args

`os.Args` 是进程启动时运行时填好的命令行参数，类型是 `[]string`。下标 0 是这次用来启动的程序路径，从下标 1 起才是写在程序名后面的参数。

Go 不按空格切开，也不认识 `--http` 这种选项。边界是启动这个进程的那一方定的：在终端里就是壳。程序看到的只是一串已经切好的字符串。

在本目录执行：

```sh
go run . --dev --http console
```

只用标准库。前四节用写死的切片，输出每次相同。最后一节才是这次进程真正的 `os.Args`。下面按打印顺序说明，完整输出在文末。

## 1. 下标 0 是程序，后面才是参数

学习计划里启动开发链的那一行，切完是四个字符串：

```go
args := []string{"./build/bin/geth", "--dev", "--http", "console"}
```

`[0]` 是程序路径。`--dev`、`--http`、`console` 都是用户参数，程序自己不会把 `--` 开头的当成特殊语法。

`node/config.go` 用这个路径的文件名当默认节点名和 IPC 文件名。Windows 上可执行文件以 `.exe` 结尾，所以先 `filepath.Base`，再去掉这个后缀：

```go
name := strings.TrimSuffix(filepath.Base(args[0]), ".exe")
```

`./build/bin/geth` 和 `geth.exe` 得到的都是 `geth`。

## 2. [1:] 和原来的切片共用底层数组

`minichain` 的 `main` 不自己解析，把程序名后面的一段交给 `run`：

```go
rest := args[1:]
```

`rest` 的长度是 2，内容是 `--dev` 和 `--http`。它没有复制底层数组。`rest[0] = "--syncmode"` 写的是同一块内存，原来的 `args[1]` 也变成 `--syncmode`。

要留给后面的函数一份改了也不影响调用方的参数，用 `slices.Clone`，或者把新切片赋回去，不要改元素。

## 3. Go 不再切开这些字符串

这三行是壳已经切完之后的样子：

```go
[]string{"./build/bin/geth", "--http.api", "eth,net,web3"} // 空格切开，三个元素
[]string{"./build/bin/geth", "--http.api=eth,net,web3"}    // 等号留在同一个元素里
[]string{"./build/bin/geth", "hello world", ""}            // 引号留住空格；空字符串也占一个位置
```

逗号、等号、空格都只是字符。空字符串 `""` 的长度是 0，但它仍然是切片里的一项。把 `--http.api=eth,net` 拆成选项名和值，是 `flag` 或 `urfave/cli` 的工作。本目录旁边的 `urfavecli` 做的就是这件事，它读的还是这一串。

## 4. 先看长度，再按第一个参数分发

`build/ci.go` 用 `[1]` 当子命令，`[2:]` 留给这个子命令。只有程序名时长度是 1，读 `[1]` 会 panic。先看长度：

```go
if len(args) < 2 {
    return "need subcommand as first argument"
}
switch args[1] {
case "test", "lint":
    // args[2:] 是这个子命令自己的参数
default:
    return "unknown command " + args[1]
}
```

`ci` 停在长度检查。`ci test -short` 分到 `test`，剩下 `[-short]`。`ci lint` 剩下的是空切片 `[]`。`ci nope` 落到未知命令。

跳过长度检查直接读 `short[1]`，panic 文本是 `index out of range [1] with length 1`。

正常启动时 `os.Args` 至少有程序名这一项。测试里会整份换掉它，例如 `minichain/cmd/node/main_test.go` 写成 `[]string{"node", "-nope"}`，结束时再换回去。它是包级变量，换掉之后这个进程里后面读到的都是新的。

## 5. 这次进程真正的 os.Args

前四节的切片是写死的。这一节读的是运行时填好的 `os.Args`。

`go run` 先编译出一个临时程序再执行，所以 `[0]` 是那份临时文件的路径，每次不一样。程序只打印 `filepath.Base` 得到的文件名。在本目录用 `go run .` 时，文件名是 `args`。

上面的命令会打印：

```text
len = 4
程序名 = args
参数 = []string{"--dev", "--http", "console"}
```

不写后面的参数时，长度是 1，参数是空切片 `[]string{}`。

## 完整输出

```text

=== 1. 下标 0 是程序，后面才是参数 ===
len = 4
[0] "./build/bin/geth"
[1] "--dev"
[2] "--http"
[3] "console"
Base 再去掉 .exe = "geth"
geth.exe 同样得到 "geth"

=== 2. [1:] 和原来的切片共用底层数组 ===
交给 run 的是 []string{"--dev", "--http"}
改 rest[0] 之后 args = []string{"./build/bin/geth", "--syncmode", "--http"}

=== 3. Go 不再切开这些字符串 ===
空格切开：     []string{"./build/bin/geth", "--http.api", "eth,net,web3"}
等号留在一起： []string{"./build/bin/geth", "--http.api=eth,net,web3"}
引号留住空格： []string{"./build/bin/geth", "hello world", ""}

=== 4. 先看长度，再按第一个参数分发 ===
need subcommand as first argument
命令 test，其余 [-short]
命令 lint，其余 []
unknown command nope
不检查直接读 [1]: panic: runtime error: index out of range [1] with length 1

=== 5. 这次进程真正的 os.Args ===
len = 4
程序名 = args
参数 = []string{"--dev", "--http", "console"}
```
