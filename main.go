package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"log"
	"strings"

	"github.com/wzshiming/gotype"
	"github.com/wzshiming/namecase"
)

func main() {
	imp := gotype.NewImporter()
	fSet := token.NewFileSet()
	//解析go文件
	f, err := parser.ParseFile(fSet, "./testdata/test_struct.go", nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}
	tp, err := imp.ImportFile("model", f)
	if err != nil {
		log.Println(err)
	}
	res := make(map[string]logStruct, 0)
	for i := 0; i < tp.NumChild(); i++ {
		v := tp.Child(i)
		if v.Kind() != gotype.Struct {
			continue
		}
		res[v.Name()] = logStruct{
			name: v.Name(),
			fs:   getStructFields(v),
		}
	}
	tmp := `func (l %s) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
%s
return nil
}
`

	rs := make([]string, 0, len(res))
	for name, l := range res {
		rs = append(rs, fmt.Sprintf(tmp, "*"+name, strings.Join(genLogMarshal("l.", l), "\n")))
	}
	fmt.Println(rs)
}

type logStruct struct {
	name string
	fs   []field
}

type field struct {
	name  string
	tag   string
	fType string
	inner []field
}

var (
	// []byte 单独使用ByteString
	baseType = []string{
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64",
		"string", "bool", "complex64", "complex128",
	}
)

func genBaseType() {
	tmp := `func (l %s) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
%s
return nil
}`

	baseTmp := `for i := range l {
		arr.Append%s(l[i])
	}`
	baseRes := make([]string, 0)
	for _, s := range baseType {
		tp := namecase.ToPascal(s) + "Array"
		tpDefine := fmt.Sprintf("\n type %s []%s \n", tp, s)
		baseRes = append(baseRes, tpDefine, fmt.Sprintf(tmp, tp, fmt.Sprintf(baseTmp, namecase.ToPascal(s))))
	}
	fmt.Println(baseRes)
	return
}

func genStructFields(prefix string, fs []field) []string {
	res := make([]string, 0, len(fs))
	for _, l := range fs {
		tp := l.fType
		val := prefix + l.name
		switch {
		case tp == "struct":
			tp = "Object"
			val = "&" + val
		case tp == "inner":
			obj := `_ = encoder.AddObject("%s", log.Ms(func(encoder zapcore.ObjectEncoder) error {
    %s
    return nil
}))`
			res = append(res, fmt.Sprintf(obj, namecase.ToLowerSnake(l.name), strings.Join(genStructFields(prefix+l.name+".", l.inner), "\n")))
			continue
		case strings.HasPrefix(tp, "[]") && tp != "[]byte":
			tp2 := strings.TrimPrefix(tp, "[]")
			if isBaseType(tp2) {
				val = fmt.Sprintf("log.%sArray(%s)", namecase.ToPascal(tp2), val)
			} else {
				// 结构体数组使用func
				genObjArray(l.name)
			}
			tp = "Array"
		case tp == "[]byte":
			tp = "ByteString"
		case tp == "time.Time":
			tp = "Time"
		case tp == "time.Duration":
			tp = "Duration"
		case isBaseType(tp):

		default:
			fmt.Println("unknown type", tp)
			continue
		}
		res = append(res, fmt.Sprintf(`encoder.Add%s("%s", %s)`, namecase.ToPascal(tp), namecase.ToLowerSnake(l.name), val))
	}
	return res
}

func genLogMarshal(prefix string, l logStruct) []string {
	return genStructFields(prefix, l.fs)
}

func genObjArray(name string) string {
	return fmt.Sprintf(`log.ObjArray(func(encoder zapcore.ArrayEncoder) error {
            for i := range l {
                err := encoder.AppendObject(&%s)
                if err != nil {
                    return err
                }
            }
          return nil
		})`, name)
}

func isBaseType(t string) bool {
	for _, s := range baseType {
		if t == s {
			return true
		}
	}
	return false
}

func getStructFields(v gotype.Type) []field {
	res := make([]field, 0)
	if v.Kind() != gotype.Struct {
		return res
	}
	for j := 0; j < v.NumField(); j++ {
		f := v.Field(j)
		switch {
		case f.IsAnonymous():
			res = append(res, field{
				name:  f.Name(),
				tag:   f.Tag().Get("json"),
				fType: "struct",
			})
		case f.Elem().Kind() == gotype.Struct:
			if f.Elem().Name() == "" {
				res = append(res, field{
					name:  f.Name(),
					tag:   f.Tag().Get("json"),
					fType: "inner",
					inner: getStructFields(f.Elem()),
				})
				continue
			}
			if f.Elem().String() == "time.Time" {
				res = append(res, field{
					name:  f.Name(),
					tag:   f.Tag().Get("json"),
					fType: f.Elem().String(),
				})
				continue
			}
			res = append(res, field{
				name:  f.Name(),
				tag:   f.Tag().Get("json"),
				fType: "struct",
				inner: getStructFields(f.Elem()),
			})
		default:
			res = append(res, field{
				name:  f.Name(),
				tag:   f.Tag().Get("json"),
				fType: f.Elem().String(),
			})
		}
	}
	return res
}
