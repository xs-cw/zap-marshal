package src

import "time"

//go:generate go run github.com/xs-cw/zap-marshal Request Response RespResult
type Request struct {
	Param struct {
		Type  string
		Num   int
		Index []string
		Inner []struct {
			S [][]string
		}
	}
	Filter struct {
		Type string
	}
	Time  time.Time
	Map   map[string]string
	Map2  map[string][]string
	Name  string
	Flag  rune
	Flags []rune
	UID   string
}

type Response struct {
	Result RespResult
	ErrMsg string
	ErrNO  int
}

type RespResult struct {
	Type string
	Num  int
	List map[string][]string
}
