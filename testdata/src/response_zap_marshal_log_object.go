package src

import (
	"go.uber.org/zap/zapcore"
)

func (l Response) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddObject("result", l.Result)
	encoder.AddString("err_msg", l.ErrMsg)
	return nil
}
