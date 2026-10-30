package main

// Account 的字段顺序就是编码顺序。note 未导出，生成器和反射都会跳过它。
//
// 下面这行对编译器来说是普通注释。go build、go run、go test 都不会执行它。
// 只有单独运行 go generate 时，go 命令才在这个目录里执行注释后面的程序，
// 并把 -type、-out 原样当成那个程序的参数。
//
//go:generate go run ./gen -type Account -out gen_account.go
type Account struct {
	Nonce   uint64
	Balance uint64
	Name    string
	Code    []byte
	note    string
}

// Transfer 没有 //go:generate，也没有 EncodeGen。
// 类型缓存见到它时会编译一份反射 writer，而不是调用生成的方法。
type Transfer struct {
	Amount uint64
	Note   string
}
