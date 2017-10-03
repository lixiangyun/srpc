package srpc

type Stat struct {
	RecvCnt int64
	SendCnt int64
	ErrCnt  int64
}

func (s1 Stat) Sub(s2 Stat) Stat {
	s1.RecvCnt -= s2.RecvCnt
	s1.SendCnt -= s2.SendCnt
	s1.ErrCnt -= s2.ErrCnt
	return s1
}

var stat Stat

func GetStat() Stat {
	return stat
}

func StatAdd(recv, send, err int64) {
	stat.RecvCnt += recv
	stat.SendCnt += send
	stat.ErrCnt += err
}
