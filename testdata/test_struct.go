package testdata

type Request struct {
	Param struct {
		Type  string
		Num   int
		Index []string
		Inner struct {
			S string
		}
	}
	Filter struct {
		Type string
	}
	Name string
	UID  string
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
