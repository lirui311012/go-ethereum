// 反射教程：运行时查看类型、读写值、调用方法，以及把检查结果编译成闭包。
//
// 普通函数在编译期就知道参数是哪个结构体。rlp.Encode、JSON、RPC 注册面对的是调用方
// 传进来的任意值，字段和 tag 要等程序跑起来才看得到。reflect 就是这条路。
//
// 每一节对应 README 里的同名步骤。在本目录执行 go run . 看输出。
package main

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
)

func main() {
	typeAndValue()
	namedType()
	setValue()
	structTags()
	collections()
	callMethods()
	interfaceDynamic()
	typeCacheDemo()
}

func section(n int, title string) {
	fmt.Printf("\n=== %d. %s ===\n", n, title)
}

// Account 是后面几节共用的例子。tag 的写法和编码库看到的一样。
type Account struct {
	Address string `json:"address" rlp:"-"`
	Balance int    `json:"balance"`
	secret  int
}

func typeAndValue() {
	section(1, "Type、Value、Kind")

	acct := Account{Address: "0xabc", Balance: 10, secret: 7}

	// TypeOf / ValueOf 的参数类型是 any，也就是 interface{}。
	// 调用 TypeOf(acct) 时，Go 先把 acct 装进一个接口值，再传进函数。
	// 接口值里有两样东西：动态类型（这里是 Account）和动态值（这份结构体数据）。
	// TypeOf 读动态类型，所以得到 main.Account，Kind 是 struct，才能接着 NumField。
	// 参数声明上的 any 本身能装进任何具体类型，它自己没有字段。
	// 函数若返回这个声明类型，每次调用都是 interface{}，看不出传进来的是什么。
	// 第 7 步里什么都没装的接口没有动态类型，TypeOf 得到 nil。
	t := reflect.TypeOf(acct)
	v := reflect.ValueOf(acct)
	fmt.Printf("Type = %s\n", t)
	fmt.Printf("Kind = %s\n", t.Kind())
	fmt.Printf("Value.Kind = %s，和 Type.Kind 相同\n", v.Kind())
	fmt.Printf("字段数 = %d\n", t.NumField())

	// 没有值、只有类型时用 TypeFor。accounts/abi 里对 big.Int 就是这样取类型的。
	fmt.Printf("TypeFor[Account] = %s\n", reflect.TypeFor[Account]())
	fmt.Printf("TypeFor[*Account] = %s，Elem = %s\n", reflect.TypeFor[*Account](), reflect.TypeFor[*Account]().Elem())

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		fv := v.Field(i)
		exported := f.PkgPath == ""
		fmt.Printf("  [%d] %s %s exported=%v", i, f.Name, f.Type, exported)
		fmt.Printf(" PkgPath = %s", f.PkgPath)
		// 未导出字段不能 Interface，否则 panic。CanInterface 先问一声。
		if fv.CanInterface() {
			fmt.Printf(" value=%v", fv.Interface())
		} else {
			fmt.Printf(" value=(未导出，不能 Interface)")
		}
		fmt.Println()
	}
}

// Wei 的底层种类是 int64，类型名字仍是 Wei。Kind 回答「怎么存放」，Type 回答「叫什么」。
type Wei int64

func namedType() {
	section(2, "具名类型和底层种类")

	var w Wei = 100
	t := reflect.TypeOf(w)
	fmt.Printf("Type = %s，Name = %q，Kind = %s，PkgPath = %s\n", t, t.Name(), t.Kind(), t.PkgPath())

	plain := reflect.TypeOf(int64(0))
	fmt.Printf("和 int64 是同一个类型: %v\n", t == plain)
	fmt.Printf("可以转换成 int64: %v\n", t.ConvertibleTo(plain))

	asInt := reflect.ValueOf(w).Convert(plain)
	fmt.Printf("Convert 之后的类型 = %s，值 = %d\n", asInt.Type(), asInt.Int())
}

func setValue() {
	section(3, "改值：地址、Elem、CanSet")

	acct := Account{Address: "0xabc", Balance: 10}

	// ValueOf 拿到的是副本。这份副本没有地址，写了也回到不了 acct。
	copied := reflect.ValueOf(acct)
	fmt.Printf("值副本 CanAddr=%v CanSet=%v Kind=%s Type=%s\n", copied.CanAddr(), copied.CanSet(), copied.Kind(), copied.Type())

	// 传入指针，Elem 解开一层，得到原来那个变量。
	elem := reflect.ValueOf(&acct).Elem()
	fmt.Printf("指针再 Elem CanAddr=%v CanSet=%v Kind=%s Type=%s\n", elem.CanAddr(), elem.CanSet(), elem.Kind(), elem.Type())

	balance := elem.FieldByName("Balance")
	fmt.Printf("Balance CanSet=%v，改之前 = %d\n", balance.CanSet(), balance.Int())
	balance.SetInt(20)
	fmt.Printf("改之后 acct.Balance = %d\n", acct.Balance)

	// 未导出字段即使在本包里，反射也当作包外调用：CanSet 为 false。
	secret := elem.FieldByName("secret")
	fmt.Printf("secret CanSet=%v CanInterface=%v\n", secret.CanSet(), secret.CanInterface())

	// 种类不匹配时 SetInt 会 panic。先看 Kind，再选 SetInt / SetString / Set。
	addr := elem.FieldByName("Address")
	fmt.Printf("改之前 acct.Address = %s\n", acct.Address)
	if addr.Kind() == reflect.String && addr.CanSet() {
		addr.SetString("0xdef")
	}
	fmt.Printf("改之后 acct.Address = %s\n", acct.Address)
}

// Tx 用来演示 tag。Hash 不参与 RLP；Input 的 tag 只是记下来，本示例不实现 tail 的语义。
type Tx struct {
	Nonce uint64 `json:"nonce"`
	Hash  string `json:"-" rlp:"-"`
	Input []byte `json:"input,omitempty" rlp:"tail"`
}

// Identity 嵌进 User 后，Name 可以从外层用名字找到，下标是一条路径。
type Identity struct {
	Name string
}

type User struct {
	Identity
	Age int `json:"age,omitempty"`
}

func structTags() {
	section(4, "结构体字段和 tag")

	t := reflect.TypeFor[Tx]()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		jsonTag, hasJSON := f.Tag.Lookup("json")
		rlpTag := f.Tag.Get("rlp")
		fmt.Printf("  %s json=%q (存在=%v) rlp=%q\n", f.Name, jsonTag, hasJSON, rlpTag)
		if hasJSON {
			name, opt, hasOpt := strings.Cut(jsonTag, ",")
			fmt.Printf("    json 名字=%q", name)
			if hasOpt {
				fmt.Printf(" 选项=%q", opt)
			}
			fmt.Println()
		}
	}

	u := User{Identity: Identity{Name: "alice"}, Age: 3}
	uv := reflect.ValueOf(u)
	ut := uv.Type()

	// NumField 不把嵌入展平。第 0 个字段是整个 Identity。
	fmt.Printf("User 的字段数 = %d，第 0 个是 %s，Anonymous=%v\n", ut.NumField(), ut.Field(0).Name, ut.Field(0).Anonymous)

	// FieldByName 会走进嵌入字段。Index 是从外到内的下标路径。
	nameField, ok := ut.FieldByName("Name")
	fmt.Printf("FieldByName(Name) ok=%v Index=%v\n", ok, nameField.Index)
	fmt.Printf("沿 Index 读到的值 = %s\n", uv.FieldByIndex(nameField.Index).String())
	fmt.Printf("FieldByName 直接读 = %s\n", uv.FieldByName("Name").String())
}

func collections() {
	section(5, "切片、map，以及按类型构造")

	nums := []int{1, 2}
	sv := reflect.ValueOf(&nums).Elem()
	fmt.Printf("切片 len=%d cap=%d IsNil=%v\n", sv.Len(), sv.Cap(), sv.IsNil())
	// Append 返回新的切片头。要让 nums 变长，得 Set 回去。
	sv.Set(reflect.Append(sv, reflect.ValueOf(3)))
	fmt.Printf("Append 之后 nums = %v\n", nums)

	var empty []int
	fmt.Printf("nil 切片 IsNil=%v len=%d\n", reflect.ValueOf(empty).IsNil(), reflect.ValueOf(empty).Len())

	// map 的值里放的是指针式的头，改内容不用再取地址。
	m := map[string]int{"eth": 1}
	mv := reflect.ValueOf(m)
	mv.SetMapIndex(reflect.ValueOf("blob"), reflect.ValueOf(2))
	fmt.Printf("SetMapIndex 之后 = %v\n", m)
	fmt.Printf("key 类型 = %s，elem 类型 = %s\n", mv.Type().Key(), mv.Type().Elem())

	// 只有类型、还没有值时，用 MakeSlice / MakeMap。
	sliceType := reflect.TypeFor[[]string]()
	made := reflect.MakeSlice(sliceType, 0, 2)
	made = reflect.Append(made, reflect.ValueOf("a"), reflect.ValueOf("b"))
	fmt.Printf("MakeSlice = %v\n", made.Interface())

	mapType := reflect.TypeFor[map[string]int]()
	madeMap := reflect.MakeMap(mapType)
	madeMap.SetMapIndex(reflect.ValueOf("k"), reflect.ValueOf(1))
	fmt.Printf("MakeMap = %v\n", madeMap.Interface())
}

// Wallet 的方法集用来区分两种取方法的方式。
// 值方法同时出现在 Wallet 和 *Wallet 上；指针方法只出现在 *Wallet 上。
type Wallet struct {
	Addr string
}

func (w Wallet) Label() string {
	return "wallet " + w.Addr
}

func (w *Wallet) SetAddr(addr string) {
	w.Addr = addr
}

func callMethods() {
	section(6, "调用方法")

	w := Wallet{Addr: "0x1"}
	fmt.Printf("Wallet 的导出方法数 = %d\n", reflect.TypeOf(w).NumMethod())   // 1
	fmt.Printf("*Wallet 的导出方法数 = %d\n", reflect.TypeOf(&w).NumMethod()) // 2

	// Value.MethodByName 已经绑好接收者。Call 只传方法自己的参数。
	bound := reflect.ValueOf(&w).MethodByName("SetAddr")
	fmt.Printf("绑定后的类型 = %s，IsValid=%v\n", bound.Type(), bound.IsValid())
	bound.Call([]reflect.Value{reflect.ValueOf("0x2")})
	fmt.Printf("调用 SetAddr 之后 Addr = %s\n", w.Addr)

	label := reflect.ValueOf(w).MethodByName("Label").Call(nil)
	fmt.Printf("Label() = %s\n", label[0].String())

	// 值上没有指针方法。IsValid 为 false，不能 Call。
	missing := reflect.ValueOf(w).MethodByName("SetAddr")
	fmt.Printf("值上的 SetAddr IsValid=%v\n", missing.IsValid())

	// Type.Method 拿出的 Func 把接收者放在第一个参数。
	// rpc/service.go 用的是这一条：callback 分开保存 rcvr 和 fn，调用时再把 rcvr 接到参数前面。
	method, ok := reflect.TypeOf(&w).MethodByName("SetAddr")
	fmt.Printf("Type.Method 找到=%v，函数类型 = %s\n", ok, method.Type)
	fmt.Printf("  In(0) 接收者 = %s，In(1) = %s\n", method.Type.In(0), method.Type.In(1))
	method.Func.Call([]reflect.Value{reflect.ValueOf(&w), reflect.ValueOf("0x3")})
	fmt.Printf("经 Func.Call 之后 Addr = %s\n", w.Addr)
}

func interfaceDynamic() {
	section(7, "接口里的动态类型")

	describe := func(name string, v any) {
		rv := reflect.ValueOf(v)
		if !rv.IsValid() {
			fmt.Printf("%s: 空接口，没有动态类型\n", name)
			return
		}
		nilKind := rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Slice || rv.Kind() == reflect.Map || rv.Kind() == reflect.Interface
		isNil := nilKind && rv.IsNil()
		fmt.Printf("%s: 动态类型=%s Kind=%s IsNil=%v\n", name, rv.Type(), rv.Kind(), isNil)
	}

	var empty any
	describe("空接口", empty)

	// 接口变量本身是 nil。
	var reader io.Reader
	describe("nil io.Reader", reader)

	// 接口里装了一个类型为 *bytes.Buffer 的 nil 指针。接口变量不是 nil，动态类型存在。
	var buf *bytes.Buffer
	var asReader io.Reader = buf
	describe("装着 nil *Buffer 的 io.Reader", asReader)
	fmt.Printf("asReader == nil: %v\n", asReader == nil)
}

// encoder 在生成时定好要读哪些字段。调用时只按下标取值。
type encoder func(reflect.Value) string

// fieldPlan 是生成期留下的字段计划。闭包抓住它，以后不再读 tag。
type fieldPlan struct {
	name  string
	index int
}

// typeCache 是 rlp/typecache.go 的缩小版：键是 reflect.Type，值是编译好的函数。
// 本示例用一把锁保护 map。geth 的实现把读路径从锁上挪开，键里还带了结构体 tag。
type typeCache struct {
	mu sync.Mutex
	m  map[reflect.Type]encoder
}

func (c *typeCache) encode(v any) string {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return "<nil>"
	}
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return "<nil>"
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return fmt.Sprintf("跳过非结构体 %s", rv.Type())
	}
	return c.encoderFor(rv.Type())(rv)
}

func (c *typeCache) encoderFor(t reflect.Type) encoder {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = make(map[reflect.Type]encoder)
	}
	if enc, ok := c.m[t]; ok {
		fmt.Printf("  cache hit %s\n", t)
		return enc
	}
	fmt.Printf("  cache miss，生成 %s\n", t)
	enc := compileStruct(t)
	c.m[t] = enc
	return enc
}

// compileStruct 只在第一次见到该类型时调用。rlp:"-" 和未导出字段不进入计划。
func compileStruct(t reflect.Type) encoder {
	var plans []fieldPlan
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			fmt.Printf("    跳过未导出字段 %s\n", f.Name)
			continue
		}
		if f.Tag.Get("rlp") == "-" {
			fmt.Printf("    跳过 %s（rlp:\"-\"）\n", f.Name)
			continue
		}
		fmt.Printf("    留下 %s index=%d rlp=%q\n", f.Name, i, f.Tag.Get("rlp"))
		plans = append(plans, fieldPlan{name: f.Name, index: i})
	}
	// plans 返回后不再修改，闭包可以一直用这份切片。
	return func(v reflect.Value) string {
		var b strings.Builder
		for i, p := range plans {
			if i > 0 {
				b.WriteByte(' ')
			}
			fmt.Fprintf(&b, "%s=%v", p.name, v.Field(p.index).Interface())
		}
		return b.String()
	}
}

func typeCacheDemo() {
	section(8, "类型缓存：反射只做一次")

	var c typeCache
	tx := Tx{Nonce: 1, Hash: "0xhash", Input: []byte{1, 2, 3}}
	fmt.Printf("第一次: %s\n", c.encode(tx))
	fmt.Printf("第二次: %s\n", c.encode(&Tx{Nonce: 2, Hash: "other", Input: []byte{4}}))
	fmt.Printf("另一种类型: %s\n", c.encode(Account{Address: "0xabc", Balance: 10, secret: 7}))
	fmt.Printf("Account 再来一次: %s\n", c.encode(Account{Address: "0xdef", Balance: 11, secret: 1}))
}
