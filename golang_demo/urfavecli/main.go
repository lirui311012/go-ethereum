package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
)

// flag 是带 -- 的命名选项，不是 mkdir、路径这种位置参数。
// 子命令不会继承根命令的 Flags：选项写在子命令名后面时，必须出现在该子命令自己的 Flags 里。
var (
	// 不用 -v：urfave 已经把 -v 留给 --version。
	verboseFlag = &cli.BoolFlag{
		Name:  "verbose",
		Usage: "打印将要执行的操作",
	}
	// 只给会写磁盘的子命令。ls 不挂它，所以 clidemo ls --dry-run 会报未知 flag。
	dryRunFlag = &cli.BoolFlag{
		Name:  "dry-run",
		Usage: "只打印将要做的事，不写磁盘",
	}
	allFlag = &cli.BoolFlag{
		Name:    "all",
		Aliases: []string{"a"},
		Usage:   "连隐藏文件一起列出",
	}
	longFlag = &cli.BoolFlag{
		Name:    "long",
		Aliases: []string{"l"},
		Usage:   "显示权限、大小和修改时间",
	}

	// mkdir 和 cp 共用。对应 geth 里 console 再拼上 nodeFlags 的做法。
	writeFlags = []cli.Flag{verboseFlag, dryRunFlag}

	// ===== 新增：greet 子命令专用的非 bool flag 演示 =====
	// StringFlag：字符串，带默认值
	greetNameFlag = &cli.StringFlag{
		Name:    "name",
		Aliases: []string{"n"},
		Usage:   "问候对象的名字",
		Value:   "world", // 不传时的默认值
	}
	// IntFlag：整数
	greetAgeFlag = &cli.IntFlag{
		Name:    "age",
		Aliases: []string{"a"},
		Usage:   "对方年龄（岁）",
		Value:   18,
	}
	// Float64Flag：浮点数
	greetHeightFlag = &cli.Float64Flag{
		Name:  "height",
		Usage: "身高（米）",
		Value: 1.70,
	}
	// DurationFlag：时长，自动解析 "300ms"、"1.5h"、"2s" 这类字符串
	greetTimeoutFlag = &cli.DurationFlag{
		Name:  "timeout",
		Usage: "问候超时时间，如 500ms / 2s / 1.5h",
		Value: 1 * time.Second,
	}
	// StringSliceFlag：可重复传入，或用逗号分隔。例如 --tag foo --tag bar 或 --tag foo,bar
	greetTagsFlag = &cli.StringSliceFlag{
		Name:    "tag",
		Aliases: []string{"t"},
		Usage:   "标签，可多次传入或用逗号分隔",
	}
)

func main() {
	app := &cli.App{
		Name:    "clidemo",
		Usage:   "演示 urfave/cli/v2 的根命令、子命令、flag 和 Before",
		Version: "0.1.0",
		// 根命令的选项。不写子命令时有效；写在子命令名前面时也由根命令解析，例如 clidemo --verbose mkdir work。
		Flags: []cli.Flag{verboseFlag},
		// Command.Run 解析完 flag 之后、进入子命令或根 Action 之前调用。
		// --help / --version 在这之前已经返回。
		// urfave/cli 只在 Version 非空时注册 --version；空字符串会把 HideVersion 设为 true。
		Before: func(ctx *cli.Context) error {
			fmt.Fprintln(os.Stderr, "before: flag 已解析，尚未进入子命令或根 Action")
			return nil
		},
		// 第一个位置参数命中 Commands 时走子命令，否则调用 root。
		Action: root,
		Commands: []*cli.Command{
			{
				Name:      "mkdir",
				Usage:     "创建一个新目录",
				ArgsUsage: "<path>",
				Flags:     writeFlags,
				Action:    makeDir,
			},
			{
				Name:      "ls",
				Usage:     "列出目录内容",
				ArgsUsage: "[path]",
				Flags:     []cli.Flag{verboseFlag, allFlag, longFlag},
				Action:    listDir,
			},
			{
				Name:      "cp",
				Usage:     "复制文件",
				ArgsUsage: "<src> <dst>",
				Flags:     writeFlags,
				Action:    copyFile,
			},
			{
				Name:      "greet",
				Usage:     "演示非 bool 类型的 flag（string/int/float64/duration/slice）",
				ArgsUsage: "[extra]",
				Flags: []cli.Flag{
					greetNameFlag,
					greetAgeFlag,
					greetHeightFlag,
					greetTimeoutFlag,
					greetTagsFlag,
				},
				Action: greet,
			},
		},
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// root 只在没有匹配到子命令时运行。
func root(ctx *cli.Context) error {
	fmt.Println("---- root ----")
	if args := ctx.Args().Slice(); len(args) > 0 {
		return fmt.Errorf("invalid command: %q", args[0])
	}
	fmt.Println("根命令：没有子命令时才会到这里。试试 clidemo mkdir <path>、ls [path]、cp <src> <dst>")
	if ctx.Bool(verboseFlag.Name) {
		fmt.Println("verbose: 这是根命令自己的选项，没有进入子命令")
	}
	return nil
}

func makeDir(ctx *cli.Context) error {
	fmt.Println("---- makeDir ----")
	if ctx.NArg() != 1 {
		return fmt.Errorf("用法: %s mkdir [--dry-run] [--verbose] <path>", ctx.App.Name)
	}
	path := ctx.Args().First()
	if ctx.Bool(verboseFlag.Name) {
		fmt.Printf("verbose: 将创建 %s（dry-run=%t）\n", path, ctx.Bool(dryRunFlag.Name))
	}
	if ctx.Bool(dryRunFlag.Name) {
		fmt.Printf("dry-run: 不创建目录 %s\n", path)
		return nil
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	fmt.Printf("已创建目录 %s\n", path)
	return nil
}

func listDir(ctx *cli.Context) error {
	fmt.Println("---- listDir ----")
	if ctx.NArg() > 1 {
		return fmt.Errorf("用法: %s ls [--all] [-l] [--verbose] [path]", ctx.App.Name)
	}
	path := "."
	if ctx.NArg() == 1 {
		path = ctx.Args().First()
	}
	if ctx.Bool(verboseFlag.Name) {
		fmt.Printf("verbose: 列出 %s（all=%t long=%t）\n", path, ctx.Bool(allFlag.Name), ctx.Bool(longFlag.Name))
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if !ctx.Bool(allFlag.Name) && strings.HasPrefix(name, ".") {
			continue
		}
		if entry.IsDir() {
			name += "/"
		}
		if !ctx.Bool(longFlag.Name) {
			fmt.Println(name)
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		fmt.Printf("%s %8d %s %s\n", info.Mode(), info.Size(), info.ModTime().Format("2006-01-02 15:04"), name)
	}
	return nil
}

func copyFile(ctx *cli.Context) error {
	fmt.Println("---- copyFile ----")
	if ctx.NArg() != 2 {
		return fmt.Errorf("用法: %s cp [--dry-run] [--verbose] <src> <dst>", ctx.App.Name)
	}
	srcPath, dstPath := ctx.Args().Get(0), ctx.Args().Get(1)
	if ctx.Bool(verboseFlag.Name) {
		fmt.Printf("verbose: 将复制 %s -> %s（dry-run=%t）\n", srcPath, dstPath, ctx.Bool(dryRunFlag.Name))
	}
	if ctx.Bool(dryRunFlag.Name) {
		fmt.Printf("dry-run: 不复制 %s -> %s\n", srcPath, dstPath)
		return nil
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dstPath, data, info.Mode().Perm()); err != nil {
		return err
	}
	fmt.Printf("已复制 %s -> %s\n", srcPath, dstPath)
	return nil
}

// greet：演示如何读取非 bool 类型的 flag
func greet(ctx *cli.Context) error {
	fmt.Println("---- greet ----")
	// StringFlag 用 ctx.String()
	name := ctx.String("name")
	// IntFlag 用 ctx.Int()
	age := ctx.Int("age")
	// Float64Flag 用 ctx.Float64()
	height := ctx.Float64("height")
	// DurationFlag 用 ctx.Duration()，拿到的是 time.Duration
	timeout := ctx.Duration("timeout")
	// StringSliceFlag 用 ctx.StringSlice()
	tags := ctx.StringSlice("tag")

	fmt.Printf("你好，%s！年龄 %d 岁，身高 %.2f m，超时 %v\n", name, age, height, timeout)
	if len(tags) > 0 {
		fmt.Printf("标签: %s\n", strings.Join(tags, ", "))
	}
	if extra := ctx.Args().Slice(); len(extra) > 0 {
		fmt.Printf("额外位置参数: %v\n", extra)
	}
	// 顺便演示：ctx.IsSet("name") 判断用户是否显式传了这个 flag（区分"用了默认值"和"真传了"）
	fmt.Printf("name 是否被显式传入: %t\n", ctx.IsSet("name"))
	return nil
}
