# clidemo

用 `github.com/urfave/cli/v2`（v2.27.5，与本仓库 geth 相同）写的小命令行程序。分发方式和 `cmd/geth/main.go` 一样：根命令、子命令、flag、`Before`。

flag 是带 `--` 的命名选项。`mkdir`、`work` 这种不带 `--` 的词是位置参数，用来选子命令或给路径。

`mkdir` 在给出的路径上创建一个目录。`ls` 列出目录，省略路径时用当前目录。`cp` 把源文件复制到目标路径。`greet` 不碰磁盘，专门演示非 bool 类型的 flag（string / int / float64 / duration / slice）。

| flag | 谁认 | 作用 |
|---|---|---|
| `--verbose` | 根命令、`mkdir`、`ls`、`cp` | 打印将要执行的操作 |
| `--dry-run` | 只有 `mkdir`、`cp` | 只打印，不写磁盘 |
| `--all`、`-a` | 只有 `ls` | 连隐藏文件一起列出 |
| `--long`、`-l` | 只有 `ls` | 显示权限、大小和修改时间 |
| `--name`、`-n` | 只有 `greet` | 问候对象的名字（默认 `world`） |
| `--age`、`-a` | 只有 `greet` | 对方年龄，整数（默认 18） |
| `--height` | 只有 `greet` | 身高，浮点数，单位米（默认 1.7） |
| `--timeout` | 只有 `greet` | 超时时长，如 `500ms`、`2s`、`1.5h`（默认 `1s`） |
| `--tag`、`-t` | 只有 `greet` | 标签，可重复传或用逗号分隔 |

子命令不会自动继承根命令的 flag。`--dry-run` 没挂在 `ls` 上，所以 `ls --dry-run` 会报未知 flag。

`greet` 用的是非 bool flag，必须跟值：`--name=foo` 或 `--name foo`，不能像 `--verbose` 那样单独出现。类型不匹配（例如 `--age=abc`）会由 urfave/cli 自己报 `parse error` 并退出，不用在代码里手写校验。

## 运行

在本目录执行：

```sh
go run .
go run . --verbose
go run . mkdir work
go run . mkdir --dry-run work
go run . ls
go run . ls -l
go run . ls --all
go run . ls work
go run . cp main.go work/main.go
go run . cp --dry-run main.go work/main.go
go run . greet
go run . greet -n 李昜 -a 28 --height=1.75
go run . greet -n Alice --timeout=250ms -t dev -t web3 -t go
go run . greet -n Bob -t a,b,c
go run . greet --age=abc
go run . --help
go run . mkdir --help
go run . greet --help
go run . --version
go run . nosuch
```

| 命令 | 结果 |
|---|---|
| `go run .` | 先打印 `before`，再进入根 `Action` |
| `go run . --verbose` | 根命令多打一行 `verbose` |
| `go run . mkdir work` | 先打印 `before`，再创建目录 `work` |
| `go run . mkdir --dry-run work` | 只打印将要创建的路径，不写磁盘 |
| `go run . ls` | 列出当前目录，跳过点开头的名字 |
| `go run . ls -l` | 每行带上权限、字节数和修改时间 |
| `go run . ls --all` | 连隐藏文件一起列出 |
| `go run . ls work` | 列出 `work` |
| `go run . cp main.go work/main.go` | 把 `main.go` 复制到 `work/main.go` |
| `go run . cp --dry-run main.go work/main.go` | 只打印复制路径，不写磁盘 |
| `go run . ls --dry-run` | `ls` 没有这个 flag，解析失败 |
| `go run . greet` | 全用默认值，打印 `你好，world！…`，`name 是否被显式传入: false` |
| `go run . greet -n 李昜 -a 28 --height=1.75` | string/int/float64 三种 flag 一起传，`name 是否被显式传入: true` |
| `go run . greet -n Alice --timeout=250ms -t dev -t web3 -t go` | duration 解析成 `250ms`，slice 用多次 `-t` 收集 |
| `go run . greet -n Bob -t a,b,c` | slice 也支持逗号写法 |
| `go run . greet --age=abc` | 类型不匹配，urfave/cli 直接报 `invalid value "abc" for flag -age: parse error` |
| `go run . --help` | 打印帮助。`--help` 在 `Before` 之前返回 |
| `go run . mkdir --help` | 只列出 `mkdir` 自己的 flag：`--verbose`、`--dry-run` |
| `go run . greet --help` | 列出 `greet` 自己的 5 个 flag 及默认值 |
| `go run . --version` | 打印 `clidemo version 0.1.0`。`--version` 在 `Before` 之前返回 |
| `go run . nosuch` | 没有这个子命令，落到根 `Action`，报 `invalid command` |

创建出来的 `work` 只是演示产物，可以删掉。

## 和 geth 的对应关系

`app.Run(os.Args)` 把 `Action`、`Before`、`Commands` 收成根命令（`app.go` 的 `newRootCommand`），再进 `command.go` 的 `Command.Run`：

1. 解析 flag。`--help`、`--version` 在这里返回。`--version` 只有 `App.Version` 非空时才会注册；空字符串会把 `HideVersion` 设为 true，这时传入 `--version` 会报 `flag provided but not defined`。
2. 调用 `Before`。
3. 第一个位置参数命中 `Commands`（这里是 `mkdir`、`ls`、`cp`、`greet`）就执行该子命令。子命令再解析自己的 flag。
4. 否则执行根 `Action`（这里是 `root`）。多出来的位置参数报错。

geth 里对应的是 `app.Flags`（`nodeFlags`、`rpcFlags`）和 `app.Commands` 里的 `init`、`account`、`console`。`console` 会把自己的 `Flags` 设成 `nodeFlags` 加上 `rpcFlags`；`account` 只挑自己用得到的。这里的 `writeFlags` 同样只挂在 `mkdir` 和 `cp` 上。

`greet` 额外演示了非 bool flag 的写法：`cli.StringFlag` / `cli.IntFlag` / `cli.Float64Flag` / `cli.DurationFlag` / `cli.StringSliceFlag`，Action 里分别用 `ctx.String()` / `ctx.Int()` / `ctx.Float64()` / `ctx.Duration()` / `ctx.StringSlice()` 取值；`ctx.IsSet("name")` 用来区分"用户真传了"和"用了 `Value` 字段的默认值"。geth 里的 `--datadir`（string）、`--maxpeers`（int）、`--bootnodes`（slice）就是同一套机制。
