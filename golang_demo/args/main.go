// os.Args：进程启动时，运行时放进来的命令行参数，类型是 []string。
//
// 下标 0 是这次用来启动的程序路径。从下标 1 起，才是写在程序名后面的参数。
// Go 不按空格切开，也不认识 --http 这种选项。壳已经切好，这里只是一串字符串。
//
// 前四节用写死的切片，输出每次相同。最后一节打印这次进程真正的 os.Args。
// 在本目录执行：
//
//	go run . --dev --http console
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	layout()
	shared()
	literal()
	subcommand()
	thisProcess()
}

func section(n int, title string) {
	fmt.Printf("\n=== %d. %s ===\n", n, title)
}

// layout 对照 geth 的启动行：./build/bin/geth --dev --http console。
// 四个字符串里，只有第一个是程序自己。
func layout() {
	section(1, "下标 0 是程序，后面才是参数")

	args := []string{"./build/bin/geth", "--dev", "--http", "console"}
	fmt.Printf("len = %d\n", len(args))
	for i, a := range args {
		fmt.Printf("[%d] %q\n", i, a)
	}

	// node/config.go 用程序名当默认的节点名和 IPC 文件名。Windows 上路径以 .exe 结尾。
	name := strings.TrimSuffix(filepath.Base(args[0]), ".exe")
	fmt.Printf("Base 再去掉 .exe = %q\n", name)
	fmt.Printf("geth.exe 同样得到 %q\n", strings.TrimSuffix(filepath.Base("geth.exe"), ".exe"))
}

// shared 说明 os.Args[1:] 没有复制底层数组。
// minichain 的 main 把这段交给 run；改其中某个元素，原切片的对应位置一起变。
func shared() {
	section(2, "[1:] 和原来的切片共用底层数组")

	args := []string{"./build/bin/geth", "--dev", "--http"}
	rest := args[1:]
	fmt.Printf("交给 run 的是 %#v\n", rest)

	rest[0] = "--syncmode"
	fmt.Printf("改 rest[0] 之后 args = %#v\n", args)
}

// literal 用三行对照壳已经做完的切分。等号、逗号、空格都留在字符串里。
func literal() {
	section(3, "Go 不再切开这些字符串")

	fmt.Printf("空格切开：     %#v\n", []string{"./build/bin/geth", "--http.api", "eth,net,web3"})
	fmt.Printf("等号留在一起： %#v\n", []string{"./build/bin/geth", "--http.api=eth,net,web3"})
	fmt.Printf("引号留住空格： %#v\n", []string{"./build/bin/geth", "hello world", ""})
}

// subcommand 是 build/ci.go 的形状：长度不够就停，够了用 [1] 当分发，[2:] 留给子命令。
func subcommand() {
	section(4, "先看长度，再按第一个参数分发")

	fmt.Println(dispatch([]string{"ci"}))
	fmt.Println(dispatch([]string{"ci", "test", "-short"}))
	fmt.Println(dispatch([]string{"ci", "lint"}))
	fmt.Println(dispatch([]string{"ci", "nope"}))

	short := []string{"ci"}
	fmt.Printf("不检查直接读 [1]: %s\n", catch(func() { _ = short[1] }))
}

func dispatch(args []string) string {
	if len(args) < 2 {
		return "need subcommand as first argument"
	}
	switch args[1] {
	case "test", "lint":
		return fmt.Sprintf("命令 %s，其余 %v", args[1], args[2:])
	default:
		return "unknown command " + args[1]
	}
}

// thisProcess 打的是运行时填好的 os.Args，不是上面的例子。
// go run 的第 0 个是临时二进制的路径，所以这里只留文件名。
func thisProcess() {
	section(5, "这次进程真正的 os.Args")

	fmt.Printf("len = %d\n", len(os.Args))
	fmt.Printf("程序名 = %s\n", strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe"))
	fmt.Printf("参数 = %#v\n", os.Args[1:])
}

func catch(fn func()) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = "panic: " + fmt.Sprint(r)
		}
	}()
	fn()
	return "没有 panic"
}
