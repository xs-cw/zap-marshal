package src

import (
	"go.uber.org/zap/zapcore"
)

func (l Request) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddObject("param", zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
		encoder.AddString("type", l.Param.Type)
		encoder.AddArray("index", zapcore.ArrayMarshalerFunc(func(encoder zapcore.ArrayEncoder) error {
			for _, v := range l.Param.Index {
				encoder.AppendString(v)
			}
			return nil
		}))
		encoder.AddArray("inner", zapcore.ArrayMarshalerFunc(func(encoder zapcore.ArrayEncoder) error {
			for _, v := range l.Param.Inner {
				encoder.AppendObject(zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
					encoder.AddArray("s", zapcore.ArrayMarshalerFunc(func(encoder zapcore.ArrayEncoder) error {
						for _, v := range v.S {
							encoder.AppendArray(zapcore.ArrayMarshalerFunc(func(encoder zapcore.ArrayEncoder) error {
								for _, v := range v {
									encoder.AppendString(v)
								}
								return nil
							}))
						}
						return nil
					}))
					return nil
				}))
			}
			return nil
		}))
		return nil
	}))
	encoder.AddObject("filter", zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
		encoder.AddString("type", l.Filter.Type)
		return nil
	}))
	encoder.AddObject("map", zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
		for k, v := range l.Map {
			encoder.AddString(k, v)
		}
		return nil
	}))
	encoder.AddString("name", l.Name)
	encoder.AddString("uid", l.UID)
	return nil
}
