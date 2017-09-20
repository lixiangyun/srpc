package main

import (
	"errors"
	"log"
	"os"
	"reflect"
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

		time.Sleep(time.Second)

		s2 := server.GetStat()

		s3 := s2.Sub(s1)

		log.Printf("Speed %d cnt/s , %.3f KB/s \r\n",
			s3.SendCnt, float32(s3.SendCnt)/(1024))

		s1 = s2
	}
}

func makereqblock(method string, req interface{}, rsp interface{}) (srpc.RequestBlock, error) {

	var reqblock srpc.RequestBlock
	var err error

	requestValue := reflect.ValueOf(req)
	rsponseValue := reflect.ValueOf(rsp)

	// 校验参数合法性，req必须是非指针类型，rsp必须是指针类型
	if rsponseValue.Kind() != reflect.Ptr {
		return reqblock, errors.New("parm rsp is'nt ptr type!")
	}

	if requestValue.Kind() == reflect.Ptr {
		return reqblock, errors.New("parm req is ptr type!")
	}

	rsponseValue = reflect.Indirect(rsponseValue)

	reqblock.Method = method
	reqblock.MsgId = 0
	reqblock.Parms[0] = requestValue.Type().String()
	reqblock.Parms[1] = rsponseValue.Type().String()
	reqblock.Body, err = srpc.CodePacket(req)
	if err != nil {
		return reqblock, err
	}

	return reqblock, nil
}

func test1(s *srpc.Server) {
	var a, b uint32
	a = 1

	reqblock, err := makereqblock("Add", a, &b)
	if err != nil {
		log.Println(err.Error())
		return
	}

	log.Println("bench mark test! ")

	for i := 0; i < 1000000; i++ {
		_, err := s.CallMethod(reqblock)
		if err != nil {
			log.Println(err.Error())
			return
		}
	}

	log.Println("bench mark end! ")

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

	netstat(100000)

	server.Stop()
}

func main() {
	addr := ":1234"
	args := os.Args
	if len(args) == 2 {
		addr = args[1]
	}

	log.Println("Addr ", addr)

	Server(addr)
}
