package srpc

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net"
	"reflect"
	"runtime/debug"
	"sync"
)

var SLICE_FLAG = []byte{255, 254, 253, 252}

type RequestBlock struct {
	MsgId  uint64
	Method string
	Parms  [2]string
	Body   []byte
}

type RsponseBlock struct {
	MsgId  uint64
	Method string
	Result string
	Body   []byte
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

	RspBlock chan RsponseBlock
}

func (s1 Stat) Sub(s2 Stat) Stat {
	s1.RecvCnt -= s2.RecvCnt
	s1.SendCnt -= s2.SendCnt
	s1.ErrCnt -= s2.ErrCnt

	return s1
}

func SliceCheck(buf []byte) int {
	length := len(buf)
	if length < 4 {
		return -1
	}

	for i := 0; i < length-4; i++ {
		if buf[i] == SLICE_FLAG[0] &&
			buf[i+1] == SLICE_FLAG[1] &&
			buf[i+2] == SLICE_FLAG[2] &&
			buf[i+3] == SLICE_FLAG[3] {
			return i
		}
	}

	return -1
}

func SendRequestBlock(conn net.Conn, reqblock []RequestBlock) error {
	iobuf := new(bytes.Buffer)

	for _, vblock := range reqblock {

		//fmt.Println("vblock: ", vblock)

		enc := gob.NewEncoder(iobuf)
		err := enc.Encode(vblock)
		if err != nil {
			return err
		}

		cnt, err := iobuf.Write(SLICE_FLAG)
		if cnt != len(SLICE_FLAG) {
			return errors.New("iobuf.write failed!")
		}

		if err != nil {
			return err
		}
	}

	err := FullyWrite(conn, iobuf.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func SendRsponseBlock(conn net.Conn, rspblock []RsponseBlock) error {

	iobuf := new(bytes.Buffer)

	for _, vblock := range rspblock {
		enc := gob.NewEncoder(iobuf)
		err := enc.Encode(vblock)
		if err != nil {
			return err
		}

		cnt, err := iobuf.Write(SLICE_FLAG)
		if cnt != len(SLICE_FLAG) {
			return errors.New("iobuf.write failed!")
		}

		if err != nil {
			return err
		}
	}

	err := FullyWrite(conn, iobuf.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func FullyWrite(conn net.Conn, buf []byte) error {

	totallen := len(buf)
	sendcnt := 0

	for {

		cnt, err := conn.Write(buf[sendcnt:])
		if err != nil {
			return err
		}

		if cnt <= 0 {
			return errors.New("conn write error!")
		}

		if cnt+sendcnt >= totallen {
			return nil
		}

		sendcnt += cnt
	}
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

func (s *Server) GetStat() Stat {
	return s.stat
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

	rsparray := make([]RsponseBlock, 100)

	for {

		index := 0

		rspblock := <-que
		rsparray[index] = rspblock
		index++

		num := len(que)
		if num > 50 {
			num = 50
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

func MsgProcess(conn net.Conn, s *Server, rspque chan RsponseBlock) {

	defer conn.Close()
	defer s.wait.Done()

	var buf [65535]byte
	var tmpbuf [1024]byte
	length := 0

	for {

		// 监听
		cnt, err := conn.Read(buf[length:])
		if err != nil {
			log.Println("server shutdown.")
			return
		}

		length += cnt

		for {

			if length == 0 {
				break
			}

			index := SliceCheck(buf[0:length])
			if -1 == index {
				break
			}

			copy(tmpbuf[0:], buf[0:index])

			if index+4 >= length {
				length = 0

			} else {
				copy(buf[0:], buf[index+4:length])
				length = length - (index + 4)
			}

			var reqblock RequestBlock

			// 反序列化客户端请求的报文
			err = DecodePacket(tmpbuf[0:index], &reqblock)
			if err != nil {
				log.Println(err.Error())
				//log.Println(len(tmpbuf), tmpbuf)
				//log.Println("length", length)
				//log.Println("index", index)
				//log.Println(len(tmpbuf), tmpbuf)
				continue
			}

			s.stat.RecvCnt++

			rspblock, err := s.CallMethod(reqblock)
			if err != nil {
				log.Println(err.Error())
				continue
			}

			//log.Println("Rsponse: ", rspblock)

			rspque <- rspblock

			s.stat.SendCnt++
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

	fmt.Println("wait!")

	s.wait.Wait()

	fmt.Println("wait done!")
}
