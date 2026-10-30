package main

import (
	"fmt"
	"reflect"
)

// EncodeReflect 在这次调用里查看 v 的字段，再按种类写入 w。
// 类型缓存里的 makeStructWriter 把同样的循环只做一次，之后按下标取值。
// gen_account.go 里的 EncodeGen 则在 go generate 时就把字段写进了源码。
func EncodeReflect(v any, w *Writer) error {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return fmt.Errorf("没有具体值")
	}
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return fmt.Errorf("nil 指针")
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("需要结构体，得到 %s", rv.Kind())
	}

	t := rv.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		// 空的 PkgPath 表示导出字段。未导出字段在包外通过反射取不到，这里同样跳过。
		if sf.PkgPath != "" {
			continue
		}
		if err := writeValue(rv.Field(i), w); err != nil {
			return fmt.Errorf("字段 %s: %w", sf.Name, err)
		}
	}
	return nil
}

func writeValue(fv reflect.Value, w *Writer) error {
	switch fv.Kind() {
	case reflect.Uint64:
		w.Uint64(fv.Uint())
	case reflect.String:
		w.String(fv.String())
	case reflect.Slice:
		if fv.Type().Elem().Kind() != reflect.Uint8 {
			return fmt.Errorf("不是 []byte")
		}
		w.Bytes(fv.Bytes())
	default:
		return fmt.Errorf("种类 %s 不支持", fv.Kind())
	}
	return nil
}
