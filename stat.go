package srpc

// 通过信息结构
type Stat struct {
	RecvCnt int64 // 请求接收次数的统计
	SendCnt int64 // 请求发送次数的统计
	ErrCnt  int64 // 出现错误次数的统计
}

// 两个统计差值，s1=s1-s2，并且返回s1结果
func (s1 Stat) Sub(s2 Stat) Stat {
	s1.RecvCnt -= s2.RecvCnt
	s1.SendCnt -= s2.SendCnt
	s1.ErrCnt -= s2.ErrCnt
	return s1
}

// 全局统计的统计
var stat Stat

// 获取当前全局的统计信息
func GetStat() Stat {
	return stat
}

// 累计统计的信息
func StatAdd(recv, send, err int64) {
	stat.RecvCnt += recv
	stat.SendCnt += send
	stat.ErrCnt += err
}
