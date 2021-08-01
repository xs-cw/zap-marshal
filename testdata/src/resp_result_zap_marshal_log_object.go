package src

import (
	"go.uber.org/zap/zapcore"
)

func (l RespResult) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddString("type", l.Type)
	encoder.AddObject("list", zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
		for k, v := range l.List {
			encoder.AddArray(k, zapcore.ArrayMarshalerFunc(func(encoder zapcore.ArrayEncoder) error {
				for _, v := range v {
					encoder.AppendString(v)
				}
				return nil
			}))
		}
		return nil
	}))
	return nil
}
