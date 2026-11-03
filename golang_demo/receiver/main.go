// 结构体方法的接收者：什么时候用值，什么时候用指针，以及为什么不要混用。
//
// 每一节对应 README 里的同名步骤。在本目录执行 go run . 看输出。
package main

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

func main() {
	valueReceiver()
	pointerReceiver()
	smallValue()
	mustBePointer()
	sliceAndMap()
	methodSet()
}

func section(n int, title string) {
	fmt.Printf("\n=== %d. %s ===\n", n, title)
}

// Snapshot 的方法都不改接收者该看到的那份数据，所以用值接收者。
// Add 故意写成「看起来像在修改」：它改的是副本，调用方的 n 不动。
type Snapshot struct {
	n int
}

func (s Snapshot) Add(d int) { s.n += d }

func (s Snapshot) N() int { return s.n }

// OK 不读字段。nil 指针上调用它仍然会 panic：panic 发生在调用前的解引用，不在方法体里。
func (s Snapshot) OK() bool { return true }

func valueReceiver() {
	section(1, "值接收者改的是副本")

	s := Snapshot{n: 1}
	s.Add(1)
	fmt.Printf("变量调用 Add 之后 N=%d\n", s.N())

	// 指针上也能调用值方法。编译器先 *p 复制出一份 Snapshot，再把副本传进去。
	p := &Snapshot{n: 1}
	p.Add(1)
	fmt.Printf("指针调用 Add 之后 N=%d\n", p.N())
}

// Counter 会修改自己，方法全部用指针接收者。
type Counter struct {
	n int
}

func (c *Counter) Add(d int) {
	c.n += d
}

func (c *Counter) N() int { return c.n }

func (c *Counter) Describe() string {
	if c == nil {
		return "<nil>"
	}
	return fmt.Sprintf("n=%d", c.n)
}

func pointerReceiver() {
	section(2, "指针接收者改的是原变量")

	c := Counter{n: 1}
	// c 是变量，可以取地址。c.Add(1) 等价于 (&c).Add(1)。
	c.Add(1)
	fmt.Printf("变量调用 Add 之后 N=%d\n", c.N())

	p := &Counter{n: 1}
	p.Add(1)
	fmt.Printf("指针调用 Add 之后 N=%d\n", p.N())

	// 临时值和 map 元素不能取地址，指针方法在那上面编译不过。
	// 下面三行因此只留在注释里：
	//
	//   Counter{}.Add(1)
	//   load().Add(1)
	//   counters["a"].Add(1)
	//
	// 先放进变量，或者让 map 存 *Counter：
	fresh := load()
	fresh.Add(1)
	counters := map[string]*Counter{"a": {n: 1}}
	counters["a"].Add(1)
	fmt.Printf("放进变量之后 N=%d，map 里存指针之后 N=%d\n", fresh.N(), counters["a"].N())

	// Describe 先判断 nil，调用本身成立。N 没有判断，一读字段就在方法体里 panic。
	var none *Counter
	fmt.Printf("nil 指针调用 Describe=%s\n", none.Describe())
	fmt.Printf("nil 指针调用 N: %s\n", catch(func() { none.N() }))
}

func load() Counter { return Counter{n: 1} }

// Point 很小，方法不修改调用方手里的那一份，按值传递。
// Shift 演示值接收者留不住修改；Moved 才是这种类型该有的写法：改副本，把新值返回。
type Point struct {
	X, Y int
}

func (p Point) Shift(dx, dy int) {
	p.X += dx
	p.Y += dy
}

func (p Point) Moved(dx, dy int) Point {
	p.X += dx
	p.Y += dy
	return p
}

func (p Point) Describe() string {
	return fmt.Sprintf("(%d,%d)", p.X, p.Y)
}

// Celsius 是一个机器字，语义是一份不可变的值。
type Celsius float64

func (c Celsius) String() string {
	return fmt.Sprintf("%g°C", float64(c))
}

func smallValue() {
	section(3, "小的、不修改调用方的类型用值接收者")

	p := Point{X: 1, Y: 2}
	p.Shift(3, 4)
	moved := p.Moved(3, 4)
	fmt.Printf("Shift 之后原点 %s\n", p.Describe())
	fmt.Printf("Moved 返回 %s，原点仍是 %s\n", moved.Describe(), p.Describe())

	// 值方法同时属于 Point 和 *Point。指针调用时先复制。
	pp := &Point{X: 1, Y: 2}
	fmt.Printf("指针上 Moved 返回 %s，原点仍是 %s\n", pp.Moved(3, 4).Describe(), pp.Describe())

	fmt.Printf("体温 %s\n", Celsius(36.5))
}

// Ledger 里有锁。复制一把正在使用的 sync.Mutex 是错误的，所以接收者只能是指针。
// 读余额的方法也同样用指针：这个类型已经有指针方法，方法集保持一种形状。
type Ledger struct {
	mu      sync.Mutex
	balance int
}

func (l *Ledger) Deposit(n int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.balance += n
}

func (l *Ledger) Balance() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.balance
}

func (l *Ledger) Describe() string {
	if l == nil {
		return "<nil>"
	}
	return fmt.Sprintf("balance=%d", l.Balance())
}

func mustBePointer() {
	section(4, "要修改、不能复制，或接收者很大，用指针接收者")

	var book Ledger
	book.Deposit(10)
	fmt.Printf("存入之后 %s\n", book.Describe())

	var missing *Ledger
	fmt.Printf("nil 账本 %s\n", missing.Describe())

	// nil 指针调用值方法：调用前要复制 *p，解引用直接 panic。
	// OK 的方法体没有读字段，进不去。
	var snap *Snapshot
	fmt.Printf("nil 指针调用值方法 OK: %s\n", catch(func() { snap.OK() }))
}

// List 和 Bag 为了把「改内容」和「改头」放在一起对照，同一个类型上两种接收者都有。
// 这是演示，不是推荐写法。类型一旦需要指针方法，其余方法也该用指针。
type List struct {
	Items []int
}

// Set 改的是底层数组里的元素。切片头被复制了，头里的指针仍指向同一块数组。
func (l List) Set(i, v int) { l.Items[i] = v }

// Append 把新切片头赋给副本，调用方的长度不变。
func (l List) Append(v int) { l.Items = append(l.Items, v) }

// AppendTo 写回调用方的切片头。
func (l *List) AppendTo(v int) { l.Items = append(l.Items, v) }

// Bag 的 Counts 是 map。map 变量本身是一个指针，复制结构体时这个指针被复制。
type Bag struct {
	Counts map[string]int
}

// Inc 改的是同一个 map 里的条目。
func (b Bag) Inc(k string) { b.Counts[k]++ }

// Replace 换掉的是副本里的 map 指针。
func (b Bag) Replace() { b.Counts = map[string]int{"x": 1} }

// ReplaceWith 换掉调用方的 map。
func (b *Bag) ReplaceWith() { b.Counts = map[string]int{"x": 1} }

func sliceAndMap() {
	section(5, "切片和 map：改内容看得到，换头看不到")

	list := List{Items: []int{1}}
	list.Set(0, 9)
	fmt.Printf("Set 之后 %v\n", list.Items)
	list.Append(2)
	fmt.Printf("值接收者 Append 之后 %v\n", list.Items)
	list.AppendTo(2)
	fmt.Printf("指针接收者 AppendTo 之后 %v\n", list.Items)

	bag := Bag{Counts: map[string]int{}}
	bag.Inc("a")
	fmt.Printf("Inc 之后 %v\n", bag.Counts)
	bag.Replace()
	fmt.Printf("值接收者 Replace 之后 %v\n", bag.Counts)
	bag.ReplaceWith()
	fmt.Printf("指针接收者 ReplaceWith 之后 %v\n", bag.Counts)
}

// Account 故意混用两种接收者，用来看方法集和接口。不要照这个写。
type Account struct {
	Name string
	N    int
}

func (a Account) Describe() string { return a.Name }

func (a *Account) Reset() {
	a.Name = ""
	a.N = 0
}

// Describer 只有值接收者就能实现。Resetter 必须用指针接收者才能实现。
type Describer interface {
	Describe() string
}

type Resetter interface {
	Reset()
}

func methodSet() {
	section(6, "混用会把方法集拆开")

	showMethods(Point{})
	showMethods(&Point{})
	showMethods(Counter{})
	showMethods(&Counter{})
	showMethods(Account{})
	showMethods(&Account{})

	fmt.Println()
	report[Point, Describer]("Point")
	report[*Point, Describer]("*Point")
	report[Counter, Describer]("Counter")
	report[*Counter, Describer]("*Counter")
	report[Account, Describer]("Account")
	report[*Account, Describer]("*Account")
	report[Account, Resetter]("Account")
	report[*Account, Resetter]("*Account")

	// 同一个变量：调用指针方法可以，放进接口不行。
	// 调用点会自动取地址；接口赋值只看方法集，不会帮你取地址。
	//
	//   c := Counter{n: 1}
	//   c.Add(1)                  // 可以
	//   var d Describer = c       // 不能编译：Counter 的方法集里没有 Describe
	//   var d Describer = &c      // 可以
	c := Counter{n: 1}
	c.Add(1)
	var d Describer = &c
	fmt.Printf("\n变量上 Add 之后，只有 &c 能放进 Describer: %s\n", d.Describe())

	origin := Point{X: 1, Y: 2}
	var byValue Describer = origin
	var byPointer Describer = &origin
	fmt.Printf("Point 和 *Point 都能放进 Describer: %s %s\n", byValue.Describe(), byPointer.Describe())
}

func showMethods(v any) {
	t := reflect.TypeOf(v)
	names := make([]string, t.NumMethod())
	for i := range names {
		names[i] = t.Method(i).Name
	}
	list := "（无）"
	if len(names) > 0 {
		list = strings.Join(names, " ")
	}
	fmt.Printf("%-16s %s\n", t.String(), list)
}

func report[T any, I any](name string) {
	ok := reflect.TypeFor[T]().Implements(reflect.TypeFor[I]())
	fmt.Printf("%-8s 实现 %-9s %v\n", name, reflect.TypeFor[I]().Name(), ok)
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
