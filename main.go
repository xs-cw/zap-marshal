package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"go/format"

	"github.com/wzshiming/gotype"
	"github.com/wzshiming/namecase"
)

func main() {
	imp := gotype.NewImporter()
	name := os.Args[1]
	tp, err := imp.Import(".", "")
	if err != nil {
		log.Fatal(err)
	}

	v, ok := tp.ChildByName(name)
	if !ok {
		log.Fatalf("not found %q", name)
	}

	buf := bytes.NewBuffer(nil)
	fmt.Fprintf(buf, `package %s

import (
	"go.uber.org/zap/zapcore"
)

`, tp.Name())

	genDefine(buf, "l", v)

	os.WriteFile(namecase.ToLowerSnake(v.Name())+"_zap_marshal_log_object.go", srcFmt(buf.Bytes()), 0644)
}

func srcFmt(b []byte) []byte {
	n, err := format.Source(b)
	if err != nil {
		return b
	}
	return n
}

func genDefine(buf io.Writer, prefix string, typ gotype.Type) {
	kind := typ.Kind()
	switch kind {
	case gotype.Map, gotype.Struct:
		fmt.Fprintf(buf, `func (%s %s) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
`, prefix, typ.Name())
		gen(buf, prefix, typ)
	case gotype.Array, gotype.Slice:
		fmt.Fprintf(buf, `func (%s %s) MarshalLogArray(encoder zapcore.ObjectEncoder) error {
`, prefix, typ.Name())
		gen(buf, prefix, typ)
	}
	fmt.Fprintf(buf, `return nil
}
`)
}

func gen(buf io.Writer, prefix string, typ gotype.Type) {
	kind := typ.Kind()
	switch kind {
	case gotype.Map:
		genMap(buf, prefix, typ)
	case gotype.Struct:
		genStruct(buf, prefix, typ)
	case gotype.Array, gotype.Slice:
		genSlice(buf, prefix, typ)
	}
}

func genStruct(buf io.Writer, prefix string, typ gotype.Type) {
	num := typ.NumField()
	for i := 0; i != num; i++ {
		field := typ.Field(i)
		elem := field.Elem()
		kind := elem.Kind()
		lname := namecase.ToLowerSnake(field.Name())
		if _, ok := elem.MethodByName("MarshalLogObject"); ok {
			fmt.Fprintf(buf, `encoder.AddObject(%q, %s.%s)
`, lname, prefix, field.Name())
			continue
		}
		if _, ok := elem.MethodByName("MarshalLogArray"); ok {
			fmt.Fprintf(buf, `encoder.AddArray(%q, %s.%s)
`, lname, prefix, field.Name())
			continue
		}
		switch kind {
		case gotype.String:
			if elem.Name() == strings.ToLower(kind.String()) {
				fmt.Fprintf(buf, `encoder.AddString(%q, %s.%s)
`, lname, prefix, field.Name())
			} else {
				fmt.Fprintf(buf, `encoder.AddString(%q, string(%s.%s))
`, lname, prefix, field.Name())
			}
		case gotype.Map:
			fmt.Fprintf(buf, `encoder.AddObject(%q, zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
`, lname)
			genMap(buf, prefix+"."+field.Name(), elem)
			fmt.Fprintf(buf, `return nil
}))
`)
		case gotype.Struct:
			fmt.Fprintf(buf, `encoder.AddObject(%q, zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
`, lname)
			genStruct(buf, prefix+"."+field.Name(), elem)
			fmt.Fprintf(buf, `return nil
}))
`)
		case gotype.Array, gotype.Slice:
			fmt.Fprintf(buf, `encoder.AddArray(%q, zapcore.ArrayMarshalerFunc(func(encoder zapcore.ArrayEncoder) error {
`, lname)
			genSlice(buf, prefix+"."+field.Name(), elem)
			fmt.Fprintf(buf, `return nil
}))
`)
		}
	}

}

func genSlice(buf io.Writer, prefix string, typ gotype.Type) {
	elem := typ.Elem()
	kind := elem.Kind()

	if _, ok := elem.MethodByName("MarshalLogObject"); ok {
		fmt.Fprintf(buf, `encoder.AppendObject(%s)
`, prefix)
		return
	}
	if _, ok := elem.MethodByName("MarshalLogArray"); ok {
		fmt.Fprintf(buf, `encoder.AppendArray(%s)
`, prefix)
		return
	}

	fmt.Fprintf(buf, `for _, v := range %s {
`, prefix)
	switch kind {
	case gotype.String:
		if elem.Name() == strings.ToLower(kind.String()) {
			fmt.Fprintf(buf, `encoder.AppendString(v)
`)
		} else {
			fmt.Fprintf(buf, `encoder.AppendString(string(v))
`)
		}
	case gotype.Map:
		fmt.Fprintf(buf, `encoder.AppendObject(zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
`)
		genMap(buf, "v", elem)
		fmt.Fprintf(buf, `return nil
}))
`)
	case gotype.Struct:
		fmt.Fprintf(buf, `encoder.AppendObject(zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
`)
		genStruct(buf, "v", elem)
		fmt.Fprintf(buf, `return nil
}))
`)
	case gotype.Array, gotype.Slice:
		fmt.Fprintf(buf, `encoder.AppendArray(zapcore.ArrayMarshalerFunc(func(encoder zapcore.ArrayEncoder) error {
`)
		genSlice(buf, "v", elem)
		fmt.Fprintf(buf, `return nil
}))
`)
	}

	fmt.Fprintf(buf, `}
`)
}

func genMap(buf io.Writer, prefix string, typ gotype.Type) {
	key := typ.Key()
	elem := typ.Elem()
	kind := elem.Kind()

	fmt.Fprintf(buf, `for k, v := range %s {
`, prefix)

	k := "k"
	if key.Name() != strings.ToLower(key.Kind().String()) {
		k = "sk"
	}
	switch kind {
	case gotype.String:
		if elem.Name() == strings.ToLower(kind.String()) {
			fmt.Fprintf(buf, `encoder.AddString(%s, v)
`, k)
		} else {
			fmt.Fprintf(buf, `encoder.AddString(%s, string(v))
`, k)
		}
	case gotype.Struct:
		fmt.Fprintf(buf, `encoder.AddObject(%s, zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
`, k)
		genStruct(buf, "v", elem)
		fmt.Fprintf(buf, `return nil
}))
`)
	case gotype.Array, gotype.Slice:
		fmt.Fprintf(buf, `encoder.AddArray(%s, zapcore.ArrayMarshalerFunc(func(encoder zapcore.ArrayEncoder) error {
`, k)
		genSlice(buf, "v", elem)
		fmt.Fprintf(buf, `return nil
}))
`)
	}

	fmt.Fprintf(buf, `}
`)
}
