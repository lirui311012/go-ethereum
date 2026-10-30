// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package rlp

import (
	"fmt"
	"maps"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/rlp/internal/rlpstruct"
)

// typeinfo 是某个 Go 类型编好的 RLP 编解码器，也是类型缓存里的一条记录。
//
// 反射只在第一次见到这个类型时发生：makeDecoder / makeWriter 把它编译成闭包，
// 之后每次编码或解码只调用 decoder / writer，不再重复检查种类、字段和 tag。
// 生成失败时错误留在 decoderErr / writerErr，同一种类型以后直接返回同一个错误。
type typeinfo struct {
	decoder    decoder
	decoderErr error // error from makeDecoder
	writer     writer
	writerErr  error // error from makeWriter
}

// typekey 是类型缓存的键。除了 reflect.Type，还带上结构体 tag。
//
// 同一个 Go 类型出现在不同字段上，编解码规则可以不同。rlp:"nilString" / rlp:"nilList" 决定 nil 指针
// 写成空字符串还是空列表，rlp:"tail" 决定切片是再包一层列表，还是直接接住外层列表剩下的元素。
// optional 本身不改变字段类型的函数，但它和这些 tag 绑在一起，所以也进键，避免不同字段共用一条缓存。
// 顶层 Encode / Decode 用的是空 tag。
type typekey struct {
	reflect.Type
	rlpstruct.Tags
}

// decoder 把 Stream 里的下一个 RLP 值写进 val。函数在生成期做成闭包，调用期不再反射。
type decoder func(*Stream, reflect.Value) error

// writer 把 val 的 RLP 追加进 encBuffer。和 decoder 一样，每个类型只生成一次。
type writer func(reflect.Value, *encBuffer) error

// theTC 是进程内唯一的类型缓存。查找已有类型不加锁，只有生成新类型才加锁。
var theTC = newTypeCache()

// typeCache 用写时复制把读路径从锁上挪开。
//
// cur 通过 atomic.Value 发布一张完整的 map。读方 Load 到的那张 map 之后不会再被写入，
// 所以查找不需要加锁，也不会和正在进行的生成互相踩。
//
// 生成时先在 mu 里把 cur 克隆到 next，递归填好相关类型，再把 next 原子换成新的 cur。
// mu 只串行化写方。next 只在持锁期间有效，发布后置空。
type typeCache struct {
	cur atomic.Value

	// This lock synchronizes writers.
	mu   sync.Mutex
	next map[typekey]*typeinfo // 保存了类型->对应的编解码器函数
}

// newTypeCache 让 cur 一开始就是一张空 map。info 的快路径会直接 Load，不能碰到 nil。
func newTypeCache() *typeCache {
	c := new(typeCache)
	c.cur.Store(make(map[typekey]*typeinfo))
	return c
}

// cachedDecoder 返回 typ 的解码闭包。未生成过就现做一份并放进缓存。
func cachedDecoder(typ reflect.Type) (decoder, error) {
	info := theTC.info(typ)
	return info.decoder, info.decoderErr
}

// cachedWriter 返回 typ 的编码闭包。与 cachedDecoder 共用同一条 typeinfo。
func cachedWriter(typ reflect.Type) (writer, error) {
	info := theTC.info(typ)
	return info.writer, info.writerErr
}

// info 是不加锁的查找。键里的 tag 为空，对应顶层类型，而不是某个结构体字段。
// 命中 cur 立刻返回；未命中才进 generate。那里会加锁，并处理并发下的重复生成。
func (c *typeCache) info(typ reflect.Type) *typeinfo {
	key := typekey{Type: typ}
	if info := c.cur.Load().(map[typekey]*typeinfo)[key]; info != nil {
		return info
	}

	// Not in the cache, need to generate info for this type.
	return c.generate(typ, rlpstruct.Tags{})
}

// generate 在锁内生成 typ，并把这一轮新产生的条目一次性发布出去。
//
// 加锁后要再查一次 cur。等锁期间，另一个写方可能已经生成并发布了同一个键。
// cur 里的 map 不能改：读方可能还拿着它。所以先克隆到 next，这一轮的新条目
// 只写进 next，完成后用 Store 把整张表换成 cur。克隆是浅拷贝，新旧 map 共享已有的 *typeinfo。
func (c *typeCache) generate(typ reflect.Type, tags rlpstruct.Tags) *typeinfo {
	c.mu.Lock()
	defer c.mu.Unlock()

	cur := c.cur.Load().(map[typekey]*typeinfo)
	if info := cur[typekey{typ, tags}]; info != nil {
		return info
	}

	// Copy cur to next.
	c.next = maps.Clone(cur)

	// Generate.
	info := c.infoWhileGenerating(typ, tags)

	// next -> cur
	c.cur.Store(c.next)
	c.next = nil
	return info
}

// infoWhileGenerating 在已经持有 mu、且 next 已经就绪时查找或创建类型信息。
// 结构体字段、切片元素、指针元素都走这里：不再加锁，也不在递归中途发布半成品 map。
//
// 创建时先塞进一个空的 typeinfo，再调用 generate。递归类型因此能停下来。
// 例如 type Node struct { Next *Node }，或 type List []List：展开字段或元素时会再次查到同一个键。
// 如果那时缓存里还没有条目，生成过程会无限递归。空条目让这次查找直接返回同一个指针。
//
// 编解码闭包保存的是这个 *typeinfo，调用时才读取里面的 decoder / writer。
// 生成尚未结束时这两个函数还是 nil，但闭包要等整棵类型树都填完才会被拿去用。
func (c *typeCache) infoWhileGenerating(typ reflect.Type, tags rlpstruct.Tags) *typeinfo {
	key := typekey{typ, tags}
	if info := c.next[key]; info != nil {
		return info
	}
	// Put a dummy value into the cache before generating.
	// If the generator tries to lookup itself, it will get
	// the dummy value and won't call itself recursively.
	info := new(typeinfo)
	c.next[key] = info
	info.generate(typ, tags)
	return info
}

// field 是结构体里一个参与 RLP 的字段。
// index 是 reflect 的字段下标。info 是该字段类型配上它自己的 tag 之后的缓存项。
// optional 为真时，输入列表到这里提前结束是允许的。optional 必须排在字段尾部，所以它后面的字段也同样可以缺席。
type field struct {
	index    int
	info     *typeinfo
	optional bool
}

// structFields 解析结构体中参与 RLP 的导出字段，并取到每个字段的 typeinfo。
//
// 先把 reflect 字段收成 rlpstruct.Field，由 ProcessFields 丢掉未导出字段和 rlp:"-"，
// 同时检查 optional、tail、nil 这些 tag 是否用对了位置。
// 每个留下的字段再用它自己的 tag 去 infoWhileGenerating。因此同一个字段类型，
// 配不同 tag 会得到不同的编解码器。
//
// 这里不能调用 info：那会再次抢 mu（这把锁不可重入），还会把还没生成完的 map 发布出去。
// 调用方必须已经处在 generate 之中。
func structFields(typ reflect.Type) (fields []field, err error) {
	// Convert fields to rlpstruct.Field.
	var allStructFields []rlpstruct.Field
	for i := 0; i < typ.NumField(); i++ {
		rf := typ.Field(i)
		allStructFields = append(allStructFields, rlpstruct.Field{
			Name:     rf.Name,
			Index:    i,
			Exported: rf.PkgPath == "",
			Tag:      string(rf.Tag),
			Type:     *rtypeToStructType(rf.Type, nil),
		})
	}

	// Filter/validate fields.
	structFields, structTags, err := rlpstruct.ProcessFields(allStructFields)
	if err != nil {
		if tagErr, ok := err.(rlpstruct.TagError); ok {
			tagErr.StructType = typ.String()
			return nil, tagErr
		}
		return nil, err
	}

	// Resolve typeinfo.
	for i, sf := range structFields {
		typ := typ.Field(sf.Index).Type
		tags := structTags[i]
		info := theTC.infoWhileGenerating(typ, tags)
		fields = append(fields, field{sf.Index, info, tags.Optional})
	}
	return fields, nil
}

// firstOptionalField 返回第一个带 optional 的字段下标。
//
// optional 只能成片出现在字段列表尾部。编码时从这里往后看，末尾连续的零值可以整段省掉，
// 这样旧分叉的短列表和新分叉的长列表能共用一个结构体。没有 optional 时返回 len(fields)，表示一个字段都不能省。
func firstOptionalField(fields []field) int {
	for i, f := range fields {
		if f.optional {
			return i
		}
	}
	return len(fields)
}

// structFieldError 给字段生成失败补上结构体名和字段名。
// 原始错误只知道“这个类型不能编码”，套上之后才能看出是哪个字段引起的。
type structFieldError struct {
	typ   reflect.Type
	field int
	err   error
}

func (e structFieldError) Error() string {
	return fmt.Sprintf("%v (struct field %v.%s)", e.err, e.typ, e.typ.Field(e.field).Name)
}

// generate 为这一条缓存同时做解码器和编码器。
// 两侧互不影响：某一侧失败只记在对应的 Err 字段里，调用方各自检查。
// makeDecoder / makeWriter 展开结构体、切片、指针时会回到 infoWhileGenerating，
// 所以这一步就是上面那个“先占坑、再填充”能够接住递归的地方。
func (i *typeinfo) generate(typ reflect.Type, tags rlpstruct.Tags) {
	i.decoder, i.decoderErr = makeDecoder(typ, tags)
	i.writer, i.writerErr = makeWriter(typ, tags)
}

// rtypeToStructType 把 reflect.Type 收成 rlpstruct.Type。
// rlpstruct 只用这份精简描述做字段过滤，以及决定 nil 指针默认编成字符串还是列表，从而不必依赖编码器本身。
//
// rec 记下已经展开过的类型。指针、切片、数组指回自身时直接返回已有节点，类型图就不会被无限展开。
// 这和 typeinfo 的空条目是两套防递归：这里防的是描述类型，那里防的是生成闭包。
func rtypeToStructType(typ reflect.Type, rec map[reflect.Type]*rlpstruct.Type) *rlpstruct.Type {
	k := typ.Kind()
	if k == reflect.Invalid {
		panic("invalid kind")
	}

	if prev := rec[typ]; prev != nil {
		return prev // short-circuit for recursive types
	}
	if rec == nil {
		rec = make(map[reflect.Type]*rlpstruct.Type)
	}

	t := &rlpstruct.Type{
		Name:      typ.String(),
		Kind:      k,
		IsEncoder: typ.Implements(encoderInterface),
		IsDecoder: typ.Implements(decoderInterface),
	}
	rec[typ] = t
	if k == reflect.Array || k == reflect.Slice || k == reflect.Pointer {
		t.Elem = rtypeToStructType(typ.Elem(), rec)
	}
	return t
}

// typeNilKind 决定指向 typ 的 nil 指针写成哪种空值。
//
// RLP 没有 null。nil 指针要落成空字符串（0x80）或空列表（0xC0）。
// rlp:"nilString"、rlp:"nilList" 用 tag 指定的种类。rlp:"nil" 以及没写 tag 时走元素类型的默认：
// 整数、字符串、布尔、字节切片和字节数组用字符串，其余用列表。rlp:"nil" 和默认种类相同，它另外表示解码时允许把空值解回 nil。
// *big.Int 和 *uint256.Int 不走这里：编码函数把它们的 nil 写成空字符串。
func typeNilKind(typ reflect.Type, tags rlpstruct.Tags) Kind {
	styp := rtypeToStructType(typ, nil)

	var nk rlpstruct.NilKind
	if tags.NilOK {
		nk = tags.NilKind
	} else {
		nk = styp.DefaultNilValue()
	}
	switch nk {
	case rlpstruct.NilKindString:
		return String
	case rlpstruct.NilKindList:
		return List
	default:
		panic("invalid nil kind value")
	}
}

// isUint 覆盖 uint、uint8 到 uint64，以及 uintptr。RLP 整数是无符号的，有符号整数不能编码。
func isUint(k reflect.Kind) bool {
	return k >= reflect.Uint && k <= reflect.Uintptr
}

// isByte 判断元素是不是“普通字节”。[]byte 和 [N]byte 因此编成一条 RLP 字符串，而不是字节的列表。
// uint8 若实现了 Encoder，就要按自定义规则逐个编码，不能再当成原始字节串。
func isByte(typ reflect.Type) bool {
	return typ.Kind() == reflect.Uint8 && !typ.Implements(encoderInterface)
}
