package srpc

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"runtime/debug"
	"sync"
)

type RequestBlock struct {
	MsgType uint64
	MsgId   uint64
	Method  string
	Parms   [2]string
	Body    []byte
}

type RsponseBlock struct {
	MsgType uint64
	MsgId   uint64
	Method  string
	Result  string
	Body    []byte
}

type funcinfo struct {
	function reflect.Value
	functype reflect.Type
	input    [2]reflect.Type
	output   reflect.Type
}

type Stat struct {
	RecvCnt int64
	SendCnt int64
	ErrCnt  int64
}

type Server struct {
	Addr string

	stat Stat

	symbol map[string]funcinfo
	pthis  reflect.Value

	wait sync.WaitGroup
}

func (s1 Stat) Sub(s2 Stat) Stat {
	s1.RecvCnt -= s2.RecvCnt
	s1.SendCnt -= s2.SendCnt
	s1.ErrCnt -= s2.ErrCnt

	return s1
}

// 报文序列化
func CodePacket(req interface{}) ([]byte, error) {
	iobuf := new(bytes.Buffer)

	enc := gob.NewEncoder(iobuf)

	err := enc.Encode(req)
	if err != nil {
		debug.PrintStack()
		return nil, err
	}

	return iobuf.Bytes(), nil
}

// 报文反序列化
func DecodePacket(buf []byte, rsp interface{}) error {
	iobuf := bytes.NewReader(buf)
	denc := gob.NewDecoder(iobuf)
	err := denc.Decode(rsp)

	if err != nil {
		debug.PrintStack()
	}

	return err
}

func NewServer(addr string) *Server {

	s := new(Server)

	s.Addr = addr
	s.symbol = make(map[string]funcinfo, 0)

	return s
}

func (s *Server) BindMethod(pthis interface{}) {

	//创建反射变量，注意这里需要传入ruTest变量的地址；
	//不传入地址就只能反射Routers静态定义的方法
	vfun := reflect.ValueOf(pthis)
	vtype := vfun.Type()

	s.pthis = vfun

	//读取方法数量
	num := vfun.NumMethod()

	fmt.Println("NumMethod:", num)

	//遍历路由器的方法，并将其存入控制器映射变量中
	for i := 0; i < num; i++ {

		var fun funcinfo
		fun.function = vfun.Method(i)
		fun.functype = vfun.Method(i).Type()
		funname := vtype.Method(i).Name

		if fun.functype.NumIn() != 2 {
			fmt.Printf("function %s (input parms %d) failed! \r\n", funname, fun.functype.NumIn())
			continue
		}

		if fun.functype.NumOut() != 1 {
			fmt.Printf("function %s (output parms %d) failed! \r\n", funname, fun.functype.NumOut())
			continue
		}

		fun.input[0] = fun.functype.In(0)
		fun.input[1] = fun.functype.In(1)

		// 校验参数合法性，req必须是非指针类型，rsp必须是指针类型
		if fun.input[0].Kind() == reflect.Ptr {
			fmt.Println("parm 1 must ptr type!")
			continue
		}

		if fun.input[1].Kind() != reflect.Ptr {
			fmt.Println("parm 2 must ptr type!")
			continue
		}

		fun.input[1] = fun.input[1].Elem()

		fun.output = fun.functype.Out(0)

		if fun.output.String() != "error" {
			fmt.Printf("function %s (output type %s) failed! \r\n", funname, fun.output)
			continue
		}

		s.symbol[funname] = fun

		fmt.Println("Add Method: ", funname,
			fun.input[0].String(), fun.input[1].String(), fun.output.String())
	}
}

func (s *Server) MatchMethod(method string, parms [2]string) ([]reflect.Type, error) {

	fun, b := s.symbol[method]
	if b == false {
		return nil, errors.New("can not found " + method)
	}

	for i := 0; i < 2; i++ {
		if parms[i] != fun.input[i].String() {
			errs := fmt.Sprintf("MatchMethod parm(%d) type not match : %s -> %s \r\n",
				i, parms[i], fun.input[i].String())
			return nil, errors.New(errs)
		}
	}

	return fun.input[0:], nil
}

func (s *Server) Call(method string, parms []reflect.Value) error {

	fun, b := s.symbol[method]
	if b == false {
		return errors.New("can not found " + method)
	}

	parms = fun.function.Call(parms)

	if len(parms) < 1 {
		return nil
	}

	if parms[0].Type().Name() == "error" {
		i := parms[0].Interface()
		if i != nil {
			return i.(error)
		}
	}

	return errors.New("success")
}

func (s *Server) GetStat() Stat {
	return s.stat
}

func msgprocess(conn net.Conn, s *Server) {

	defer conn.Close()
	defer s.wait.Done()

	var buf [4096]byte

	for {

		var reqblock RequestBlock

		// 监听
		n, err := conn.Read(buf[0:])
		if err != nil {
			log.Println("server shutdown.")
			return
		}

		s.stat.RecvCnt++

		// 反序列化客户端请求的报文
		err = DecodePacket(buf[:n], &reqblock)
		if err != nil {
			log.Println(err.Error())
			s.stat.ErrCnt++
			continue
		}

		//log.Println("Request: ", reqblock)

		parmtype, err := s.MatchMethod(reqblock.Method, reqblock.Parms)
		if err != nil {
			log.Println(err.Error())
			s.stat.ErrCnt++
			continue
		}

		var parms [2]reflect.Value

		parms[0] = reflect.New(parmtype[0])
		parms[1] = reflect.New(parmtype[1])

		err = DecodePacket(reqblock.Body, parms[0].Interface())
		if err != nil {
			log.Println(err.Error())
			s.stat.ErrCnt++
			continue
		}

		var input [2]reflect.Value

		input[0] = reflect.Indirect(parms[0])
		input[1] = parms[1]

		err = s.Call(reqblock.Method, input[0:])

		var rspblock RsponseBlock

		rspblock.MsgType = reqblock.MsgType
		rspblock.MsgId = reqblock.MsgId
		rspblock.Method = reqblock.Method
		rspblock.Result = err.Error()

		rspblock.Body, err = CodePacket(reflect.Indirect(parms[1]).Interface())
		if err != nil {
			log.Println(err.Error())
			s.stat.ErrCnt++
			continue
		}

		//log.Println("Rsponse: ", rspblock)

		rspBuf, err := CodePacket(rspblock)
		if err != nil {
			log.Println(err.Error())
			s.stat.ErrCnt++
			continue
		}

		// 将序列化后的报文发送到客户端
		_, err = conn.Write(rspBuf)
		if err != nil {

			if err == io.EOF {
				log.Println("server shutdown.")
				return
			}

			log.Println(err.Error())

			s.stat.ErrCnt++

			return
		}

		s.stat.SendCnt++
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
			conn, err2 := listen.Accept()
			if err2 != nil {
				fmt.Println(err.Error())
				continue
			}
			s.wait.Add(1)
			go msgprocess(conn, s)
		}
	}()

	return nil
}

func (s *Server) Stop() {

	fmt.Println("wait!")

	s.wait.Wait()

	fmt.Println("wait done!")
}
