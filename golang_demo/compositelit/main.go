// 复合字面量：用花括号一次性写出结构体、数组、切片、map 的值。
//
// 起点是这一行：
//
//	a := Account{Address: Address{19: 1}, Balance: 5, Nonce: 1}
//
// 外层 Account 按字段名填。冒号左边的 Address 是字段名；右边的 Address
// 是类型 [20]byte，{19: 1} 按数组下标写，不是再填一个结构体。
// 每一节对应 README 里的同名步骤。
// 在本目录执行 go run . 看输出。
package main

import "fmt"

func main() {
	keyedStruct()
	arrayIndex()
	nestedElide()
	omittedZero()
	sliceIndex()
	mapLiteral()
	pointerLiteral()
	unkeyedStruct()
}

func section(n int, title string) {
	fmt.Printf("\n=== %d. %s ===\n", n, title)
}

// Address 是 20 字节。打印成十六进制时，下标 0 在最左边，19 在最右边。
type Address [20]byte

func (a Address) String() string {
	return fmt.Sprintf("%x", a[:])
}

type Account struct {
	Address Address
	Balance int
	Nonce   uint64
}

func keyedStruct() {
	section(1, "结构体按字段名写")

	a := Account{Address: Address{19: 1}, Balance: 5, Nonce: 1}
	reordered := Account{Nonce: 1, Balance: 5, Address: Address{19: 1}}
	fmt.Printf("a.Address = %s\n", a.Address)
	fmt.Printf("a.Balance = %d\n", a.Balance)
	fmt.Printf("a.Nonce   = %d\n", a.Nonce)
	fmt.Printf("字段顺序调换后相等 %v\n", a == reordered)
}

func arrayIndex() {
	section(2, "数组按下标写")

	full := Address{19: 1}
	fmt.Printf("Address{19: 1}          = %s\n", full)
	fmt.Printf("最后一个字节 full[19]   = %d\n", full[19])
	fmt.Printf("第一个字节   full[0]    = %d\n", full[0])

	sparse := Address{0: 0xaa, 19: 1}
	fmt.Printf("Address{0: 0xaa, 19: 1} = %s\n", sparse)

	positional := Address{1, 2, 3}
	fmt.Printf("Address{1, 2, 3}        = %s\n", positional)
}

func nestedElide() {
	section(3, "切片和 map 里可以省略内层类型名")

	// 结构体字段必须写出类型。下面这行编译不过：
	//
	//   Account{Address: {19: 1}, Balance: 5, Nonce: 1}
	//
	a := Account{Address: Address{19: 1}, Balance: 5, Nonce: 1}
	fmt.Printf("结构体字段写出类型 Address = %s\n", a.Address)

	accounts := []Account{
		{Address: Address{19: 1}, Balance: 5, Nonce: 1},
		{Address: Address{19: 2}, Balance: 8, Nonce: 0},
	}
	fmt.Printf("[]Account 省略了 Account，余额 = %d %d\n", accounts[0].Balance, accounts[1].Balance)

	balances := map[Address]int{
		{19: 1}: 5,
		{19: 2}: 10,
	}
	fmt.Printf("map 键省略了 Address，...01 的余额 = %d\n", balances[Address{19: 1}])
}

func omittedZero() {
	section(4, "没写的字段是零值")

	onlyBalance := Account{Balance: 5}
	fmt.Printf("只写 Balance：Address = %s Balance = %d Nonce = %d\n",
		onlyBalance.Address, onlyBalance.Balance, onlyBalance.Nonce)

	zero := Account{}
	fmt.Printf("空花括号：    Address = %s Balance = %d Nonce = %d\n",
		zero.Address, zero.Balance, zero.Nonce)
}

func sliceIndex() {
	section(5, "切片也可以按下标写")

	packed := []int{1, 2, 3}
	fmt.Printf("[]int{1, 2, 3}     = %v  len=%d cap=%d\n", packed, len(packed), cap(packed))

	keyed := []int{2: 9}
	fmt.Printf("[]int{2: 9}        = %v  len=%d cap=%d\n", keyed, len(keyed), cap(keyed))

	mixed := []int{1, 4: 5, 6}
	fmt.Printf("[]int{1, 4: 5, 6}  = %v  len=%d cap=%d\n", mixed, len(mixed), cap(mixed))

	arr := [4]int{1: 7, 3: 8}
	fmt.Printf("[4]int{1: 7, 3: 8} = %v  len=%d cap=%d\n", arr, len(arr), cap(arr))
}

func mapLiteral() {
	section(6, "map 必须按键写")

	balances := map[Address]int{
		{19: 1}: 5,
		{19: 2}: 10,
	}
	fmt.Printf("地址 ...01 的余额 = %d\n", balances[Address{19: 1}])
	fmt.Printf("地址 ...02 的余额 = %d\n", balances[Address{19: 2}])
	fmt.Printf("不存在的地址余额 = %d\n", balances[Address{}])
	fmt.Printf("条目数 = %d\n", len(balances))
}

func pointerLiteral() {
	section(7, "取地址得到指针")

	p := &Account{Address: Address{19: 1}, Balance: 5, Nonce: 1}
	fmt.Printf("p 的类型是 *Account，Balance = %d\n", p.Balance)

	p.Balance = 9
	fmt.Printf("改字段之后 Balance = %d\n", p.Balance)

	list := []*Account{
		{Balance: 1},
		{Balance: 2},
	}
	fmt.Printf("切片元素省略了 &Account，余额 = %d %d\n", list[0].Balance, list[1].Balance)
}

func unkeyedStruct() {
	section(8, "按字段顺序写可以，少用")

	positional := Account{Address{19: 1}, 5, 1}
	keyed := Account{Address: Address{19: 1}, Balance: 5, Nonce: 1}
	fmt.Printf("按顺序     Address = %s Balance = %d Nonce = %d\n", positional.Address, positional.Balance, positional.Nonce)
	fmt.Printf("按字段名   Address = %s Balance = %d Nonce = %d\n", keyed.Address, keyed.Balance, keyed.Nonce)
	fmt.Printf("两个值相等 %v\n", positional == keyed)
}
