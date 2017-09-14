package main

import (
	"log"
	"srpc"
	//"os"
	"runtime"
	"time"
)

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

func Server(addr string) {

	var s SAVE

	s.tmp = 100

	cpunum := runtime.NumCPU()
	runtime.GOMAXPROCS(cpunum)

	log.Println("max cpu num: ", cpunum)

	//f, _ := os.Create("profile_file")
	//pprof.StartCPUProfile(f)     // 开始cpu profile，结果写到文件f中
	//defer pprof.StopCPUProfile() // 结束profile

	server := srpc.NewServer(addr)
	server.BindMethod(&s)

	err := server.Start()
	if err != nil {
		log.Println(err.Error())
		return
	}

	/*
		for i := 0; i < 100; i++ {
			time.Sleep(time.Second)
		}
	*/

	for {
		time.Sleep(time.Second)
	}

	server.Stop()
}

func main() {

	Server(":1234")
}
