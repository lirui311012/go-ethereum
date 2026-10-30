# Go 反射

`reflect` 让程序在运行时查看一个值的类型，读它、改它、调用它的方法。

普通函数的参数类型在编译期就定了。`rlp.Encode`、`encoding/json`、RPC 注册面对的是调用方传进来的任意值：结构体有哪些字段、tag 怎么写、方法是值接收者还是指针接收者，都要等跑起来才能知道。本目录的程序把这些规则分成 8 步，每步打印出能直接对照的结果。

三个对象贯穿全程：

| 对象 | 怎么拿到 | 回答的问题 |
|---|---|---|
| `reflect.Type` | `reflect.TypeOf(x)`、`reflect.TypeFor[T]()` | 这是什么类型：名字、种类、字段、方法、tag |
| `reflect.Value` | `reflect.ValueOf(x)` | 这个值现在是什么，能不能改，能不能当函数调用 |
| `Kind` | `Type.Kind()`、`Value.Kind()` | 底层怎么存放：`struct`、`int64`、`ptr`、`slice`、`map`…… |

`TypeOf` 和 `ValueOf` 的参数类型是 `any`。把具体值传进去时，Go 会把它装进接口。函数看到的是接口里的动态类型。`TypeFor[Account]()` 不需要一个值，仓库里 `accounts/abi/reflect.go` 对 `big.Int` 就是这样取类型的。

## 运行

在本目录执行：

```sh
go run .
```

只用标准库。下面每节都按程序的打印顺序说明规则，完整输出在文末。

## 1. Type、Value、Kind

`Account` 有两个导出字段和一个未导出字段 `secret`。

`Type.Kind()` 和 `Value.Kind()` 都是 `struct`。`NumField` 把未导出字段也算进去，所以字段数是 3。导出与否看 `StructField.PkgPath`：导出字段的 `PkgPath` 是空字符串。

未导出字段的 `Value` 拿得到，`CanInterface` 却是 false。这时调用 `Interface()` 会 panic。先问 `CanInterface`，再取接口值。

`TypeFor[*Account]()` 的种类是指针，`Elem()` 才是 `Account`。指针、切片、map、channel 都用 `Elem()` 看里面那一层；map 的键用 `Key()`。

## 2. 具名类型和底层种类

`type Wei int64` 之后，`Wei` 和 `int64` 是两种类型，`==` 比较类型得到 false。它们的 `Kind` 都是 `int64`。

`Kind` 回答值怎么存放，`Type` 回答它叫什么、属于哪个包。`Name()` 是 `Wei`，`PkgPath()` 是 `main`。内建类型 `int64` 的名字就是 `int64`，包路径为空。

`ConvertibleTo` 为 true 时，`Convert` 得到一个新的 `reflect.Value`，类型变成目标类型。整数之间的转换走的是这条，结构体字段之间不会因为字段长得像就自动转换。

## 3. 改值

`ValueOf(acct)` 拿到的是装进接口时的副本。这份副本没有地址，`CanAddr` 和 `CanSet` 都是 false。对它 `SetInt` 也写不回原来的变量。

要改 `acct`，传入 `&acct`，再 `Elem()` 解开指针。得到的 `Value` 指向原来的变量，`CanSet` 为 true，`SetInt` / `SetString` 会直接改 `acct`。

`CanSet` 还要求字段导出。`secret` 即使声明在本包里，反射也按包外调用处理，`CanSet` 和 `CanInterface` 都是 false。

`SetInt` 只接受整数种类。种类不对会 panic。先看 `Kind`，再选 `SetInt`、`SetString` 或 `Set`。

## 4. 结构体字段和 tag

`StructField.Tag` 的类型是 `reflect.StructTag`。

- `Get("rlp")` 取出这个 key 的值。没写这个 key 时返回空字符串。`Tx.Nonce` 没有 `rlp` tag，所以是 `""`。`Tx.Hash` 写了 `rlp:"-"`，所以是 `"-"`。
- `Lookup("json")` 多返回一个 bool，用来区分「没写这个 tag」和「tag 的值是空的」。

`json:"input,omitempty"` 里，逗号前面是名字，后面是选项。程序用 `strings.Cut` 切开。`json:"-"` 这个 tag 是存在的，名字本身是 `-`。`encoding/json` 把这个名字理解成跳过该字段。

嵌入字段不会被 `NumField` 展平。`User` 的第 0 个字段是整个 `Identity`，`Anonymous` 为 true。`FieldByName("Name")` 会走进去，返回的 `Index` 是 `[0 0]`：外层第 0 个字段，再取里面第 0 个字段。`FieldByIndex` 顺着这条路径取值。`Value.FieldByName("Name")` 走的是同一条路径。

## 5. 切片、map，以及按类型构造

切片的头（指针、长度、容量）要写回去，所以传入的是 `&nums`，`Elem()` 之后才 `Set`。`Append` 返回新的头，不修改原来的 `Value`。

nil 切片的 `IsNil` 为 true，`Len` 为 0。长度为 0 的非 nil 切片 `IsNil` 为 false。这两种值编码时经常要分开处理。

map 变量里放的是指向内部结构的头，`ValueOf(m)` 就可以 `SetMapIndex`。键和元素的类型用 `Type.Key()`、`Type.Elem()`。

手里只有 `reflect.Type`、还没有值时，用 `MakeSlice`、`MakeMap` 造一个零元素的值，再 `Append` 或 `SetMapIndex`。

## 6. 调用方法

`NumMethod` 只数导出方法。

`Wallet` 上有值方法 `Label`。`*Wallet` 上有 `Label` 和指针方法 `SetAddr`，所以方法数分别是 1 和 2。值上取 `SetAddr`，`IsValid` 为 false，这时不能 `Call`。

有两种拿方法的方式：

| | `Value.MethodByName` | `Type.MethodByName(...).Func` |
|---|---|---|
| 接收者 | 已经绑在这个 `Value` 上 | 函数的第 0 个参数 |
| `Call` 的参数 | 只传方法自己的参数 | 先传接收者，再传方法参数 |
| `SetAddr` 的类型 | `func(string)` | `func(*Wallet, string)` |

`rpc/service.go` 用的是第二种。`suitableCallbacks` 遍历 `receiver.Type().Method`，把 `method.Func` 和接收者分开存进 `callback`。`makeArgTypes` 跳过第 0 个参数（接收者），调用时再把接收者接到参数列表前面。

## 7. 接口里的动态类型

`reflect.ValueOf` 面对一个真正的空接口时，返回的 `Value` 是无效的，`IsValid` 为 false。类型为 `io.Reader` 的 nil 变量也是这种情况：接口里既没有动态类型，也没有动态值。

```go
var buf *bytes.Buffer
var asReader io.Reader = buf
```

这里接口里有动态类型 `*bytes.Buffer`，动态值是 nil。`asReader == nil` 为 false，因为接口的类型字段不是空的。`Value.IsNil` 为 true，因为指针是 nil。

`IsNil` 只对指针、切片、map、channel、函数、接口这些种类有意义，对结构体或整数调用会 panic。

## 8. 类型缓存

前 7 步每次都重新问类型：数字段、读 tag、查方法。同一种结构体编码一万次，这些答案不会变。

`typeCache` 把答案收成一个 `encoder` 闭包。第一次见到 `Tx` 时遍历字段，跳过未导出字段和 `rlp:"-"`，把剩下的名字和下标放进 `fieldPlan`。闭包只按下标读值。`Hash` 因此不会出现在输出里。第二次再编码 `Tx`，打印 `cache hit`，不再遍历字段。

`Account` 是另一个 `reflect.Type`，缓存键不同，会再生成一次。`Address` 标了 `rlp:"-"`，输出里只剩 `Balance`。

这是 `rlp/typecache.go` 的缩小版，有三处故意没做：

1. 这里的键只有 `reflect.Type`。geth 的键是 `typekey`，里面还有 `rlpstruct.Tags`。同一个 Go 类型挂在不同字段上时，`rlp:"tail"` 和 `rlp:"nil"` 要各生成一套闭包。
2. 这里每次查找都拿 `sync.Mutex`。geth 用 `atomic.Value` 发布整张 map，已经生成过的类型在读路径上不加锁。
3. 闭包里仍然通过 `reflect.Value` 取字段。区块头这类热路径由 `rlp/rlpgen` 生成具体的 `EncodeRLP`，调用时不再经过反射。`core/types/gen_header_rlp.go` 就是生成结果。

生成递归类型时，geth 会先往缓存里放一个空的 `typeinfo`，再展开字段。`type Node struct { Next *Node }` 因此会查到同一个键并停下来。本示例的 `compileStruct` 不递归进入字段类型，所以没有这一步。

## 和 go-ethereum 的对应关系

| 本示例 | 仓库里的位置 |
|---|---|
| `Type`、`Kind`、字段、tag | `rlp/typecache.go` 的 `structFields`：用 `PkgPath == ""` 判断导出，把 `Tag` 交给 `rlpstruct.ProcessFields` |
| 生成一次、之后调用闭包 | 同文件的 `typeinfo.decoder` / `writer`。`makeDecoder`、`makeWriter` 按 `Kind` 分派 |
| `Type.Method` 与接收者参数 | `rpc/service.go` 的 `suitableCallbacks`、`callback.call` |
| `TypeFor`、`Convert` | `accounts/abi/reflect.go` 的 `ConvertType` |

## 完整输出

```
=== 1. Type、Value、Kind ===
Type = main.Account
Kind = struct
Value.Kind = struct，和 Type.Kind 相同
字段数 = 3
TypeFor[Account] = main.Account
TypeFor[*Account] = *main.Account，Elem = main.Account
  [0] Address string exported=true PkgPath = value=0xabc
  [1] Balance int exported=true PkgPath = value=10
  [2] secret int exported=false PkgPath = main value=(未导出，不能 Interface)

=== 2. 具名类型和底层种类 ===
Type = main.Wei，Name = "Wei"，Kind = int64，PkgPath = main
和 int64 是同一个类型: false
可以转换成 int64: true
Convert 之后的类型 = int64，值 = 100

=== 3. 改值：地址、Elem、CanSet ===
值副本 CanAddr=false CanSet=false Kind=struct Type=main.Account
指针再 Elem CanAddr=true CanSet=true Kind=struct Type=main.Account
Balance CanSet=true，改之前 = 10
改之后 acct.Balance = 20
secret CanSet=false CanInterface=false
改之前 acct.Address = 0xabc
改之后 acct.Address = 0xdef

=== 4. 结构体字段和 tag ===
  Nonce json="nonce" (存在=true) rlp=""
    json 名字="nonce"
  Hash json="-" (存在=true) rlp="-"
    json 名字="-"
  Input json="input,omitempty" (存在=true) rlp="tail"
    json 名字="input" 选项="omitempty"
User 的字段数 = 2，第 0 个是 Identity，Anonymous=true
FieldByName(Name) ok=true Index=[0 0]
沿 Index 读到的值 = alice
FieldByName 直接读 = alice

=== 5. 切片、map，以及按类型构造 ===
切片 len=2 cap=2 IsNil=false
Append 之后 nums = [1 2 3]
nil 切片 IsNil=true len=0
SetMapIndex 之后 = map[blob:2 eth:1]
key 类型 = string，elem 类型 = int
MakeSlice = [a b]
MakeMap = map[k:1]

=== 6. 调用方法 ===
Wallet 的导出方法数 = 1
*Wallet 的导出方法数 = 2
绑定后的类型 = func(string)，IsValid=true
调用 SetAddr 之后 Addr = 0x2
Label() = wallet 0x2
值上的 SetAddr IsValid=false
Type.Method 找到=true，函数类型 = func(*main.Wallet, string)
  In(0) 接收者 = *main.Wallet，In(1) = string
经 Func.Call 之后 Addr = 0x3

=== 7. 接口里的动态类型 ===
空接口: 空接口，没有动态类型
nil io.Reader: 空接口，没有动态类型
装着 nil *Buffer 的 io.Reader: 动态类型=*bytes.Buffer Kind=ptr IsNil=true
asReader == nil: false

=== 8. 类型缓存：反射只做一次 ===
  cache miss，生成 main.Tx
    留下 Nonce index=0 rlp=""
    跳过 Hash（rlp:"-"）
    留下 Input index=2 rlp="tail"
第一次: Nonce=1 Input=[1 2 3]
  cache hit main.Tx
第二次: Nonce=2 Input=[4]
  cache miss，生成 main.Account
    跳过 Address（rlp:"-"）
    留下 Balance index=1 rlp=""
    跳过未导出字段 secret
另一种类型: Balance=10
  cache hit main.Account
Account 再来一次: Balance=11
```
