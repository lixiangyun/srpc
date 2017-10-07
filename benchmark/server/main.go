package main

import (
	"log"
	"os"
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

func netstat() {

	s1 := srpc.GetStat()
	log.Println("start stat...")

	for {

		time.Sleep(time.Second)
		s2 := srpc.GetStat()
		s3 := s2.Sub(s1)
		s1 = s2

		log.Printf("Throughput %.3f kTPS \r\n", float32(s3.SendCnt)/(1024))
	}
}

func Server(addr string) {

	var s SAVE

	s.tmp = 100

	server = srpc.NewServer(addr)
	server.RegMethod(&s)

	go server.Start()

	netstat()
}

func main() {

	cpunum := runtime.NumCPU()
	runtime.GOMAXPROCS(cpunum)

	log.Println("max cpu num: ", cpunum)

	addr := ":1234"
	args := os.Args
	if len(args) == 2 {
		addr = args[1]
	}
	log.Println("Addr ", addr)
	Server(addr)
}
