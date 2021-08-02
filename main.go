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

func init() {
	log.SetFlags(log.Lshortfile)
}

var imp = gotype.NewImporter()
var baseTypes = []gotype.Kind{
	gotype.Bool,
	gotype.Int,
	gotype.Int8,
	gotype.Int16,
	gotype.Int32,
	gotype.Int64,
	gotype.Uint,
	gotype.Uint8,
	gotype.Uint16,
	gotype.Uint32,
	gotype.Uint64,
	gotype.Uintptr,
	gotype.Float32,
	gotype.Float64,
	gotype.Complex64,
	gotype.Complex128,
	gotype.String,
	gotype.Byte,
	gotype.Rune,
}
var (
	objectEncoder gotype.Type
)

func main() {
	zapcore, err := imp.Import("go.uber.org/zap/zapcore", "")
	if err != nil {
		log.Fatal(err)
	}
	e, ok := zapcore.ChildByName("ObjectMarshaler")
	if !ok {
		log.Fatalf("not found ObjectEncoder")
	}
	objectEncoder = e

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
	fmt.Fprintf(buf, "package %s \n import ( \n \"go.uber.org/zap/zapcore\" \n)\n", tp.Name())

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
		fmt.Fprintf(buf, "func (%s %s) MarshalLogObject(encoder zapcore.ObjectEncoder) error {\n", prefix, typ.Name())
		gen(buf, prefix, typ)
	case gotype.Array, gotype.Slice:
		fmt.Fprintf(buf, "func (%s %s) MarshalLogArray(encoder zapcore.ObjectEncoder) error {\n", prefix, typ.Name())
		gen(buf, prefix, typ)
	}
	fmt.Fprintf(buf, "return nil \n}\n")
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
		logName := namecase.ToLowerSnake(field.Name())

		if gotype.Implements(elem, objectEncoder) {
			fmt.Fprintf(buf, "encoder.AddObject(%q, %s.%s)\n", logName, prefix, field.Name())
			continue
		}

		if _, ok := elem.MethodByName("MarshalLogArray"); ok {
			fmt.Fprintf(buf, "encoder.AddArray(%q, %s.%s)\n", logName, prefix, field.Name())
			continue
		}
		if elem.PkgPath() == "time" {
			switch elem.Name() {
			case "Time":
				fmt.Fprintf(buf, "encoder.AddTime(%q, %s.%s)\n", logName, prefix, field.Name())
				continue
			}
		}

		switch kind {
		default:
			if isBaseType(kind) {
				genStructBaseType(buf, logName, prefix+"."+field.Name(), elem)
			} else {
				log.Println("unexpect type", kind.String())
			}
		case gotype.Map:
			fmt.Fprintf(buf, "encoder.AddObject(%q, zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {\n", logName)
			genMap(buf, prefix+"."+field.Name(), elem)
			fmt.Fprintf(buf, "return nil \n}))\n")
		case gotype.Struct:
			fmt.Fprintf(buf, "encoder.AddObject(%q, zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {\n", logName)
			genStruct(buf, prefix+"."+field.Name(), elem)
			fmt.Fprintf(buf, "return nil\n}))\n")
		case gotype.Array, gotype.Slice:
			if isBytesType(elem) {
				genObjectBytesType(buf, logName, prefix+"."+field.Name(), elem)
			} else {
				fmt.Fprintf(buf, "encoder.AddArray(%q, zapcore.ArrayMarshalerFunc(func(encoder zapcore.ArrayEncoder) error {\n", logName)
				genSlice(buf, prefix+"."+field.Name(), elem)
				fmt.Fprintf(buf, "return nil\n}))\n")
			}
		}
	}

}

func genSlice(buf io.Writer, prefix string, typ gotype.Type) {
	elem := typ.Elem()
	kind := elem.Kind()

	if _, ok := elem.MethodByName("MarshalLogObject"); ok {
		fmt.Fprintf(buf, "encoder.AppendObject(%s)\n", prefix)
		return
	}
	if _, ok := elem.MethodByName("MarshalLogArray"); ok {
		fmt.Fprintf(buf, "encoder.AppendArray(%s)\n", prefix)
		return
	}

	fmt.Fprintf(buf, "for _, v := range %s {\n", prefix)
	switch kind {
	default:
		if isBaseType(kind) {
			genSliceBaseType(buf, "v", elem)
		} else {
			log.Println("unexpect type", kind.String())
		}
	case gotype.Map:
		fmt.Fprintf(buf, "encoder.AppendObject(zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {\n")
		genMap(buf, "v", elem)
		fmt.Fprintf(buf, "return nil \n}))\n")
	case gotype.Struct:
		fmt.Fprintf(buf, "encoder.AppendObject(zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {\n")
		genStruct(buf, "v", elem)
		fmt.Fprintf(buf, "return nil \n}))\n")
	case gotype.Array, gotype.Slice:
		if isBytesType(elem) {
			genSliceBytesType(buf, "v", elem)
		} else {
			fmt.Fprintf(buf, "encoder.AppendArray(zapcore.ArrayMarshalerFunc(func(encoder zapcore.ArrayEncoder) error {\n")
			genSlice(buf, "v", elem)
			fmt.Fprintf(buf, "return nil \n}))\n")
		}
	}
	fmt.Fprintf(buf, "}\n")
}

func genMap(buf io.Writer, prefix string, typ gotype.Type) {
	key := typ.Key()
	elem := typ.Elem()
	kind := elem.Kind()
	fmt.Fprintf(buf, "for k, v := range %s {\n", prefix)
	k := "k"
	if key.Name() != strings.ToLower(key.Kind().String()) {
		k = keyTypeConvert(k, key.Kind())
	}
	switch kind {
	default:
		if isBaseType(kind) {
			genMapBaseType(buf, k, "v", elem)
		} else {
			log.Println("unexpect type", kind.String())
		}
	case gotype.Struct:
		fmt.Fprintf(buf, "encoder.AddObject(%s, zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {\n", k)
		genStruct(buf, "v", elem)
		fmt.Fprintf(buf, "\n return nil \n}))\n")
	case gotype.Array, gotype.Slice:
		if isBytesType(elem) {
			genObjectBytesType(buf, k, "v", elem)
		} else {
			fmt.Fprintf(buf, "encoder.AddArray(%s, zapcore.ArrayMarshalerFunc(func(encoder zapcore.ArrayEncoder) error {\n", k)
			genSlice(buf, "v", elem)
			fmt.Fprintf(buf, "return nil \n}))\n")
		}
	}

	fmt.Fprintf(buf, "}\n")
}
func isBytesType(tp gotype.Type) bool {
	return (tp.Kind() == gotype.Slice || tp.Kind() == gotype.Array) && (tp.Elem().Kind() == gotype.Byte)
}
func genObjectBytesType(buf io.Writer, logName string, val string, tp gotype.Type) {
	switch tp.Kind() {
	case gotype.Slice:
		fmt.Fprintf(buf, "encoder.AddByteString(%q, %s)\n", logName, val)
	case gotype.Array:
		fmt.Fprintf(buf, "encoder.AddByteString(%q, %s[:])\n", logName, val)
	default:
		log.Println("unexpect type", tp.Name(), tp.Kind().String())
	}
}

func isBaseType(p gotype.Kind) bool {
	for _, baseType := range baseTypes {
		if p == baseType {
			return true
		}
	}
	return false
}

func genStructBaseType(buf io.Writer, logName string, val string, tp gotype.Type) {
	kind := tp.Kind()
	tpName := tp.Name()
	if kind == gotype.Rune {
		kind = gotype.String
	}
	if tpName == strings.ToLower(kind.String()) {
		fmt.Fprintf(buf, "encoder.Add%s(%q, %s)\n", namecase.ToPascal(tpName), logName, val)
	} else {
		fmt.Fprintf(buf, "encoder.Add%s(%q, %s(%s))\n", namecase.ToPascal(kind.String()), logName, strings.ToLower(kind.String()), val)
	}
}

func genMapBaseType(buf io.Writer, logName string, val string, tp gotype.Type) {
	kind := tp.Kind()
	tpName := tp.Name()
	if kind == gotype.Rune {
		kind = gotype.String
	}
	if tpName == strings.ToLower(kind.String()) {
		fmt.Fprintf(buf, "encoder.Add%s(%s, %s)\n", namecase.ToPascal(tpName), logName, val)
	} else {
		fmt.Fprintf(buf, "encoder.Add%s(%s, %s(%s))\n", namecase.ToPascal(kind.String()), logName, strings.ToLower(kind.String()), val)
	}
}

func genSliceBaseType(buf io.Writer, val string, tp gotype.Type) {
	kind := tp.Kind()
	tpName := tp.Name()
	if kind == gotype.Rune {
		kind = gotype.String
	}
	if tpName == strings.ToLower(kind.String()) {
		fmt.Fprintf(buf, "encoder.Append%s(%s)\n", namecase.ToPascal(tpName), val)
	} else {
		fmt.Fprintf(buf, "encoder.Append%s(%s(%s))\n", namecase.ToPascal(kind.String()), strings.ToLower(kind.String()), val)
	}
}

func genSliceBytesType(buf io.Writer, val string, tp gotype.Type) {
	switch tp.Kind() {
	case gotype.Slice:
		fmt.Fprintf(buf, "encoder.AppendByteString(%s)\n", val)
	case gotype.Array:
		fmt.Fprintf(buf, "encoder.AppendByteString(%s[:])\n", val)
	default:
		log.Println("unexpect type", tp.Name(), tp.Kind().String())
	}
}

func keyTypeConvert(key string, kind gotype.Kind) string {
	switch kind {
	case gotype.String, gotype.Rune:
		return fmt.Sprintf("string(%s)", key)
	case gotype.Int, gotype.Int8, gotype.Int16, gotype.Int32, gotype.Int64:
		return fmt.Sprintf("strconv.FormatInt(int64(%s),10)", key)
	case gotype.Uint, gotype.Uint8, gotype.Uint16, gotype.Uint32, gotype.Uint64:
		return fmt.Sprintf("strconv.FormatUint(uint64(%s))", key)
	case gotype.Float32, gotype.Float64:
		return fmt.Sprintf("strconv.FormatFloat(float64(%s),'f',-1,64)", key)
	case gotype.Complex64, gotype.Complex128:
		return fmt.Sprintf("strconv.FormatComplex(complex128(%s),'f',-1,64)", key)
	case gotype.Bool:
		return fmt.Sprintf("strconv.FormatBool(%s)", key)
	default:
		return fmt.Sprintf("fmt.Sprint(%s)", key)
	}
}
