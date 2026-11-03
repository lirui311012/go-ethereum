# 结构体方法：值接收者还是指针接收者

方法写成 `func (p Point) M()` 还是 `func (p *Point) M()`，先看这三件事：

- 方法要修改调用方手里的那份值，用指针接收者。
- 结构体不能被复制，或者已经有指针方法，这个类型的方法全部用指针接收者。
- 类型很小，方法不修改调用方的值，语义就是「一份值」，用值接收者。拿不准时用指针。

同一个结构体上两种接收者可以同时存在，编译器也允许。不要混用。有一个方法必须是指针接收者时，其余方法也写成指针接收者，这样 `T` 和 `*T` 的方法集不会裂成两半。

这是 [Go FAQ](https://go.dev/doc/faq#methods_on_values_or_pointers) 和 [代码评审约定](https://go.dev/wiki/CodeReviewComments#receiver-type) 里的规则。本目录的程序把每条规则跑一遍，在本目录执行：

```sh
go run .
```

只用标准库。下面按程序的打印顺序说明，完整输出在文末。

## 调用时传进去的是什么

值接收者：调用前把接收者整个复制一份，方法里看到的是副本。

指针接收者：方法拿到的是地址，写字段会写回原来的变量。

传值还是传地址，看的是方法声明上的接收者，不是调用方变量是不是指针。指针变量调用值方法，编译器先解引用、复制，再调用。可寻址的值调用指针方法，编译器先取地址，再调用。

## 1. 值接收者改的是副本

`Snapshot.Add` 的接收者是值。方法体里的 `s.n += d` 改的是这份副本。

```go
type Snapshot struct {
	n int
}

func (s Snapshot) Add(d int) { s.n += d }

func (s Snapshot) N() int { return s.n }
```

变量 `s` 从 1 加 1，加完仍是 1。换成指针再调一次，还是 1：`p.Add(1)` 会先做成 `(*p).Add(1)`，复制发生在取到值之后。

要修改调用方的字段，这个签名就写错了。值接收者适合「读，或者改完副本再返回新值」。

## 2. 指针接收者改的是原变量

`Counter` 的方法全部用指针。`Add` 写的就是调用方那一份。

```go
type Counter struct {
	n int
}

func (c *Counter) Add(d int) { c.n += d }
```

`c := Counter{n: 1}` 然后 `c.Add(1)`，`c` 是变量，可以取地址，编译器把它写成 `(&c).Add(1)`。`N` 变成 2。`p := &Counter{}` 再调用，同样写回 `p` 指向的结构体。

临时值和 map 元素取不了地址，指针方法编译失败：

```text
Counter{}.Add(1)          // cannot call pointer method Add on Counter
load().Add(1)             // 函数返回的是值，同样不能
counters["a"].Add(1)      // map 里存的是 Counter 时，元素也不能
```

先放进变量，或者让 map 存指针：

```go
fresh := load()
fresh.Add(1)

counters := map[string]*Counter{"a": {n: 1}}
counters["a"].Add(1)
```

`nil` 指针可以调用指针方法，调用本身成立。方法体如果先判断 `c == nil`，可以给出一个结果，`Describe` 返回 `<nil>`。方法体如果直接读字段，panic 发生在方法里面：

```text
nil 指针调用 Describe=<nil>
nil 指针调用 N: panic: runtime error: invalid memory address or nil pointer dereference
```

## 3. 小的、不修改调用方的类型用值接收者

同时满足这些条件时用值接收者：

- 方法不修改调用方手里的值。
- 接收者很小。一个机器字、几个整数，复制很便宜。
- 类型的用法就是按值传递。方法返回一个新值，原来的那份保持不动。

`Point` 就是这样。`Shift` 把修改留在副本里，原点不动。这种类型要的是 `Moved`：改副本，把新点返回。

```go
func (p Point) Shift(dx, dy int) {
	p.X += dx
	p.Y += dy
}

func (p Point) Moved(dx, dy int) Point {
	p.X += dx
	p.Y += dy
	return p
}
```

`(1,2)` 调用 `Shift(3,4)` 之后仍是 `(1,2)`。`Moved(3,4)` 返回 `(4,6)`，原点还是 `(1,2)`。

指针上调用 `Moved` 也一样。`pp.Moved(3,4)` 先复制出一份 `Point`，原点不动。接收者写成值，从指针调用也不会变成原地修改。

`Celsius` 是 `float64` 的具名类型，一个机器字，方法只读这份值，用值接收者。`fmt` 打印时会调用它的 `String`。

基本类型、切片、小结构体都属于这一类，前提是方法的语义不需要指针。标准库里的 `time.Time` 也是按值使用：方法返回新的 `Time`，不改调用方的那一个。

## 4. 要修改、不能复制，或接收者很大，用指针接收者

有一条成立，这个类型就用指针接收者，并且全部方法都用指针：

1. 方法要修改接收者。`Counter.Add`、`Ledger.Deposit` 都是。
2. 结构体里有不能复制的字段。`sync.Mutex`、`sync.WaitGroup`、`bytes.Buffer` 复制之后就不是原来那一份。值接收者会把整个结构体复制进方法，锁被复制，加锁和解锁不是同一把。`go vet` 对这种写法报 `passes lock by value`。
3. 接收者很大。结构体里如果有大数组，例如 `[4096]byte`，值接收者每次调用都复制那一块。几个 `int`、一个切片头、一个 map 指针都不大，不必为了省一次复制改用指针。
4. 需要把 `nil` 指针当成一种合法接收者，在方法里区分「没有这个对象」。

`Ledger` 带锁，也会改余额。`Deposit` 用指针。`Balance` 只是读取，同样用指针：这个类型已经有指针方法，方法集保持一种形状。

```go
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
```

`var book Ledger` 是可寻址的变量，`book.Deposit(10)` 会取地址，余额变成 10。`var missing *Ledger` 再调用 `Describe`，方法先看到 `nil`，返回 `<nil>`。

同一节里还有 `(*Snapshot).OK` 的对照。`OK` 是值接收者，方法体只有 `return true`，不读字段。`var snap *Snapshot` 然后 `snap.OK()` 仍然 panic。值方法在进入方法体之前要先求出 `*snap`，`nil` 指针在这一步就解引用了。

指针方法的 panic 在方法体里，值方法的 panic 在调用前。文本一样，时机不同。

## 5. 切片和 map：改内容看得到，换头看不到

值接收者复制的是结构体本身。切片字段复制的是切片头（底层数组指针、长度、容量），map 字段复制的是指向 map 的那个指针。底层数组和 map 本体没有被复制。

所以：

- 改元素、改条目，调用方看得到。`List.Set` 写 `Items[i]`，`Bag.Inc` 写 `Counts[k]`。
- 换掉头，调用方看不到。`append` 得到的新切片头、重新 `make` 出来的 map，赋给的是副本里的字段。

```go
func (l List) Set(i, v int) { l.Items[i] = v }
func (l List) Append(v int) { l.Items = append(l.Items, v) }
func (l *List) AppendTo(v int) { l.Items = append(l.Items, v) }
```

`[]int{1}` 经过 `Set(0, 9)` 变成 `[9]`。值接收者 `Append(2)` 之后仍是 `[9]`。指针接收者 `AppendTo(2)` 之后是 `[9 2]`。容量够不够、`append` 有没有换底层数组，都不影响前一个结论：调用方的切片头没有被赋值。

map 相同。`Inc("a")` 之后是 `map[a:1]`。值接收者 `Replace` 之后仍是 `map[a:1]`。指针接收者 `ReplaceWith` 之后才是 `map[x:1]`。

channel、函数值也是引用。复制之后收发、调用，对方看得到；把变量本身换成另一个 channel 或函数，要写回调用方就用指针接收者，或者把新值当作返回值。

接收者本身就是 map、函数或 channel 时，不要再套一层指针，它们已经是引用。接收者本身是切片，方法又不改切片头时，用值。方法要 `append` 并让调用方看到新切片，用指针接收者，或者返回新切片。

`List` 和 `Bag` 把两种接收者放在一起，是为了对照。对照完，这种类型只留指针接收者那一边。

## 6. 可以混用吗

语法允许。`Account` 上 `Describe` 是值接收者，`Reset` 是指针接收者，程序能编译、能运行。调用时，可寻址的值能调指针方法，指针能调值方法。

不要这样写。混用拆开的是方法集：

| 方法 | 在 `T` 的方法集里 | 在 `*T` 的方法集里 |
|---|---|---|
| 值接收者 | 有 | 有 |
| 指针接收者 | 没有 | 有 |

反射打出来的就是这个结果。`Point` 全部是值方法，`Point` 和 `*Point` 的方法一样。`Counter` 全部是指针方法，`Counter` 一个导出方法都没有，方法都在 `*Counter` 上。`Account` 混用之后，`Account` 只有 `Describe`，`*Account` 才有 `Describe` 和 `Reset`。

接口赋值只看方法集，不会像调用点那样自动取地址。

```go
type Describer interface{ Describe() string }
type Resetter interface{ Reset() }
```

程序用 `reflect.Type.Implements` 得到：

| 类型 | `Describer` | `Resetter` |
|---|---|---|
| `Point`、`*Point` | 实现 | 没有 `Reset` |
| `Counter` | 不实现 | |
| `*Counter` | 实现 | |
| `Account` | 实现 | 不实现 |
| `*Account` | 实现 | 实现 |

同一个变量上，两件事结果不同：

```go
c := Counter{n: 1}
c.Add(1)             // 可以。c 可寻址，编译器取地址
var d Describer = c  // 不能编译
var d Describer = &c // 可以
```

不能通过的那一行，编译器报：

```text
cannot use c (variable of struct type Counter) as Describer value in variable declaration: Counter does not implement Describer (method Describe has pointer receiver)
```

`Point` 全部是值方法，所以 `Point` 和 `*Point` 都能放进 `Describer`。调用方不用记住该传值还是该传指针。

混用还会让 `nil` 的行为跟方法走。`*Account` 为 `nil` 时，`Reset` 这种指针方法能被调用；`Describe` 这种值方法在解引用时 panic。同一个类型、同一种调用写法，有的方法安全，有的方法崩。

因此：只要有一个方法必须用指针接收者，其余方法也用指针接收者。读方法看起来「不用修改」，也跟着用指针。`Ledger.Balance` 就是这么做的。

全部方法都是值接收者的类型，继续全部用值。不要为了某一个只读方法改成值接收者。

## 怎么选

按这个顺序定一个类型，然后这个类型上的方法都照做：

1. 方法要改接收者，或者结构体里有不能复制的字段 → 指针。
2. 这个类型已经有指针方法 → 其余也用指针。
3. 类型小，方法不改调用方，按值传递就是它的用法 → 值。
4. 还拿不准 → 指针。

## 完整输出

```text

=== 1. 值接收者改的是副本 ===
变量调用 Add 之后 N=1
指针调用 Add 之后 N=1

=== 2. 指针接收者改的是原变量 ===
变量调用 Add 之后 N=2
指针调用 Add 之后 N=2
放进变量之后 N=2，map 里存指针之后 N=2
nil 指针调用 Describe=<nil>
nil 指针调用 N: panic: runtime error: invalid memory address or nil pointer dereference

=== 3. 小的、不修改调用方的类型用值接收者 ===
Shift 之后原点 (1,2)
Moved 返回 (4,6)，原点仍是 (1,2)
指针上 Moved 返回 (4,6)，原点仍是 (1,2)
体温 36.5°C

=== 4. 要修改、不能复制，或接收者很大，用指针接收者 ===
存入之后 balance=10
nil 账本 <nil>
nil 指针调用值方法 OK: panic: runtime error: invalid memory address or nil pointer dereference

=== 5. 切片和 map：改内容看得到，换头看不到 ===
Set 之后 [9]
值接收者 Append 之后 [9]
指针接收者 AppendTo 之后 [9 2]
Inc 之后 map[a:1]
值接收者 Replace 之后 map[a:1]
指针接收者 ReplaceWith 之后 map[x:1]

=== 6. 混用会把方法集拆开 ===
main.Point       Describe Moved Shift
*main.Point      Describe Moved Shift
main.Counter     （无）
*main.Counter    Add Describe N
main.Account     Describe
*main.Account    Describe Reset

Point    实现 Describer true
*Point   实现 Describer true
Counter  实现 Describer false
*Counter 实现 Describer true
Account  实现 Describer true
*Account 实现 Describer true
Account  实现 Resetter  false
*Account 实现 Resetter  true

变量上 Add 之后，只有 &c 能放进 Describer: n=2
Point 和 *Point 都能放进 Describer: (1,2) (1,2)
```
