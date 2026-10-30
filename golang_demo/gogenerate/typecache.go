package main

import (
	"fmt"
	"reflect"
	"sync"
)

// Encoder 对应 rlp.Encoder。方法在生成文件里，接收者是指针。
// *Account 有 EncodeGen，Account 这个值类型本身没有。
type Encoder interface {
	EncodeGen(*Writer)
}

// typeWriter 是某个具体类型定下来的编码函数。
// 一旦放进缓存，同一种类型后面的编码都调用它，不再重新选择。
type typeWriter func(reflect.Value, *Writer) error

// typeCache 是 rlp/typecache.go 的缩小版。
// 这里每次查找都加锁。geth 用 atomic.Value 发布整张 map，读路径不加锁。
type typeCache struct {
	mu sync.Mutex
	m  map[reflect.Type]typeWriter
}

// cache 对应 rlp 里的 theTC。进程里这一份就够演示命中和未命中。
var cache typeCache

var encoderType = reflect.TypeFor[Encoder]()

// EncodeCached 是 rlp.Encode 的入口缩小版：先按具体类型取 writer，再交给它写。
func EncodeCached(v any, w *Writer) error {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return fmt.Errorf("没有具体值")
	}
	wr, err := cache.writerFor(rv.Type())
	if err != nil {
		return err
	}
	return wr(rv, w)
}

func (c *typeCache) writerFor(typ reflect.Type) (typeWriter, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = make(map[reflect.Type]typeWriter)
	}
	if w, ok := c.m[typ]; ok {
		fmt.Printf("cache hit %s\n", typ)
		return w, nil
	}
	fmt.Printf("cache miss %s\n", typ)
	w, err := makeWriter(typ)
	if err != nil {
		return nil, err
	}
	c.m[typ] = w
	return w, nil
}

// makeWriter 在第一次见到 typ 时选择一条实现。
// 指针先解开，再看元素类型有没有 EncodeGen。有就缓存对方法的调用，没有就按字段编译反射 writer。
func makeWriter(typ reflect.Type) (typeWriter, error) {
	if typ.Kind() == reflect.Pointer {
		fmt.Printf("  %s 是指针，继续看 %s\n", typ, typ.Elem())
		elem, err := makeWriter(typ.Elem())
		if err != nil {
			return nil, err
		}
		return func(v reflect.Value, w *Writer) error {
			if v.IsNil() {
				return fmt.Errorf("nil 指针")
			}
			return elem(v.Elem(), w)
		}, nil
	}
	if reflect.PointerTo(typ).Implements(encoderType) {
		fmt.Printf("  %s 有 EncodeGen，之后只调用这个方法\n", typ)
		return func(v reflect.Value, w *Writer) error {
			if !v.CanAddr() {
				return fmt.Errorf("%s 不可寻址，EncodeGen 的接收者是指针", typ)
			}
			v.Addr().Interface().(Encoder).EncodeGen(w)
			return nil
		}, nil
	}
	fmt.Printf("  %s 没有 EncodeGen，按字段编译反射 writer\n", typ)
	return makeStructWriter(typ)
}

// makeStructWriter 只在生成这份闭包时查看字段。闭包抓住下标，调用时按下标取值。
func makeStructWriter(typ reflect.Type) (typeWriter, error) {
	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%s 不是结构体", typ)
	}
	type plan struct {
		name  string
		index int
	}
	var plans []plan
	for i := 0; i < typ.NumField(); i++ {
		sf := typ.Field(i)
		if sf.PkgPath != "" {
			fmt.Printf("  跳过未导出字段 %s\n", sf.Name)
			continue
		}
		switch sf.Type.Kind() {
		case reflect.Uint64, reflect.String:
		case reflect.Slice:
			if sf.Type.Elem().Kind() != reflect.Uint8 {
				return nil, fmt.Errorf("字段 %s 不是 []byte", sf.Name)
			}
		default:
			return nil, fmt.Errorf("字段 %s 的种类 %s 不支持", sf.Name, sf.Type.Kind())
		}
		fmt.Printf("  字段 %s %s\n", sf.Name, sf.Type)
		plans = append(plans, plan{name: sf.Name, index: i})
	}
	return func(v reflect.Value, w *Writer) error {
		for _, p := range plans {
			if err := writeValue(v.Field(p.index), w); err != nil {
				return fmt.Errorf("字段 %s: %w", p.name, err)
			}
		}
		return nil
	}, nil
}
