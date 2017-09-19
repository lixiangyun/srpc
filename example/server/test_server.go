package main

import (
	"log"
	"runtime"
	"srpc"
	"time"
)

var server *srpc.Server

type SAVE struct {
	tmp uint32
}

func (s *SAVE) Add(a uint32, b *uint32) error {

	*b = a + 1

	//log.Println("call add ", a, *b, s.tmp)

	return nil
}

func (s *SAVE) Sub(a uint32, b *uint32) error {

	*b = a - 1

	//log.Println("call sub ", a, *b, s.tmp)

	return nil
}

func netstat(cyc int) {

	s1 := server.GetStat()

	log.Println("start stat...")

	for i := 0; i < cyc; i++ {

		time.Sleep(5 * time.Second)

		s2 := server.GetStat()

		s3 := s2.Sub(s1)

		log.Printf("Speed %d cnt/s , %.3f KB/s\r\b",
			s3.SendCnt, float32(s3.SendCnt)/(1024))

		s1 = s2
	}
}

func Server(addr string) {

	var s SAVE

	s.tmp = 100

	cpunum := runtime.NumCPU()
	runtime.GOMAXPROCS(cpunum)

	log.Println("max cpu num: ", cpunum)

	//f, _ := os.Create("profile_file")
	//pprof.StartCPUProfile(f)     // 开始cpu profile，结果写到文件f中
	//defer pprof.StopCPUProfile() // 结束profile

	server = srpc.NewServer(addr)
	server.BindMethod(&s)

	err := server.Start()
	if err != nil {
		log.Println(err.Error())
		return
	}

	netstat(10000000)

	server.Stop()
}

func main() {

	Server(":1234")
}
