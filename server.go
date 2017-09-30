package srpc

import (
	"errors"
	"fmt"
	"log"
	"net"
	"reflect"
	"srpc/comm"
	"sync"
)

type ReqHeader struct {
	ReqID    uint64
	MethodID uint32
	BodySize uint32
	Body     []byte
}

type RspHeader struct {
	ReqID    uint64
	ErrNo    uint32
	BodySize uint32
	Body     []byte
}

type MethodInfo struct {
	ID   uint32
	Name string
	Req  string
	Rsp  string
}

type MethodAll struct {
	size   uint32
	method []MethodInfo
}

type MethodRef struct {
	ID        uint32
	FuncValue reflect.Value
	FuncType  reflect.Type
	ReqType   reflect.Type
	RspType   reflect.Type
}

type Server struct {
	id     uint32
	method map[uint32]MethodRef
	rwlock sync.RWMutex
	lis    *comm.Listen
}

func NewServer(addr string) *Server {

	s := new(Server)

	s.lis = comm.NewListen(addr)
	if s.lis == nil {
		return nil
	}

	s.method = make(map[uint32]MethodRef, 100)

	return s
}

func (s *Server) RegMethod(pthis interface{}) error {

	//创建反射变量，注意这里需要传入ruTest变量的地址；
	//不传入地址就只能反射Routers静态定义的方法

	vfun := reflect.ValueOf(pthis)
	vtype := vfun.Type()

	//读取方法数量
	num := vfun.NumMethod()
	fmt.Println("Total Method Num :", num)

	//遍历路由器的方法，并将其存入控制器映射变量中
	for i := 0; i < num; i++ {

		var fun MethodRef

		fun.FuncValue = vfun.Method(i)
		fun.FuncType = vfun.Method(i).Type()

		funname := vtype.Method(i).Name

		if fun.FuncType.NumIn() != 2 {
			log.Printf("function %s (input parms %d) failed! \r\n",
				funname, fun.FuncType.NumIn())
			continue
		}

		if fun.FuncType.NumOut() != 1 {
			log.Printf("function %s (output parms %d) failed! \r\n",
				funname, fun.FuncType.NumOut())
			continue
		}

		fun.ReqType = fun.FuncType.In(0)
		fun.RspType = fun.FuncType.In(1)

		// 校验参数合法性，req必须是非指针类型，rsp必须是指针类型
		if fun.ReqType.Kind() == reflect.Ptr {
			log.Println("parm 1 must ptr type!")
			continue
		}

		if fun.RspType.Kind() != reflect.Ptr {
			log.Println("parm 2 must ptr type!")
			continue
		}

		fun.RspType = fun.RspType.Elem()

		if fun.FuncType.Out(0).String() != "error" {
			fmt.Printf("function %s (output type %s) failed! \r\n",
				funname, fun.FuncType.Out(0).String())
			continue
		}

		s.method[s.id] = fun

		fmt.Println("Add Method: ",
			funname, fun.ReqType.String(), fun.RspType.String())
	}
}

func (s *Server) CallMethod(reqblock RequestBlock) (rspblock RsponseBlock, err error) {

	funcinfo, b := s.symbol[reqblock.Method]
	if b == false {
		err = errors.New("can not found method : " + reqblock.Method)
		return
	}

	for i := 0; i < 2; i++ {
		if reqblock.Parms[i] != funcinfo.input[i].String() {
			errs := fmt.Sprintf("MatchMethod parm(%d) type not match : %s -> %s \r\n",
				i, reqblock.Parms[i], funcinfo.input[i].String())
			err = errors.New(errs)
			return
		}
	}

	parmtype := funcinfo.input[0:]

	var parms [2]reflect.Value

	parms[0] = reflect.New(parmtype[0])
	parms[1] = reflect.New(parmtype[1])

	err = DecodePacket(reqblock.Body, parms[0].Interface())
	if err != nil {
		return
	}

	var input [2]reflect.Value

	input[0] = reflect.Indirect(parms[0])
	input[1] = parms[1]

	output := funcinfo.function.Call(input[0:])

	if output[0].Type().Name() == "error" {
		i := output[0].Interface()
		if i != nil {
			rspblock.Result = i.(error).Error()
		}
	} else {
		rspblock.Result = "success"
	}

	rspblock.MsgId = reqblock.MsgId
	rspblock.Method = reqblock.Method

	rspblock.Body, err = CodePacket(reflect.Indirect(parms[1]).Interface())
	if err != nil {
		return
	}

	return
}

func SendProccess(conn net.Conn, s *Server, que chan RsponseBlock) {

	defer s.wait.Done()

	rsparray := make([]RsponseBlock, 1001)

	for {

		index := 0

		rspblock := <-que
		rsparray[index] = rspblock
		index++

		num := len(que)
		if num > 1000 {
			num = 1000
		}

		for i := 0; i < num; i++ {
			rspblock := <-que
			rsparray[index] = rspblock
			index++
		}

		err := SendRsponseBlock(conn, rsparray[0:index])
		if err != nil {
			log.Println(err.Error())
			return
		}
	}
}

func (s *Server) Start() error {

	listen, err := net.Listen("tcp", s.Addr)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	go func() {
		for {
			rspque := make(chan RsponseBlock, 1000)
			conn, err2 := listen.Accept()
			if err2 != nil {
				fmt.Println(err.Error())
				continue
			}
			s.wait.Add(2)
			go MsgProcess(conn, s, rspque)
			go SendProccess(conn, s, rspque)
		}
	}()

	return nil
}

func (s *Server) Stop() {
	s.wait.Wait()
	fmt.Println("close!")
}
