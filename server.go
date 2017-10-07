package srpc

import (
	"log"
	"reflect"
	"runtime"
	"srpc/comm"
	"sync"
)

const (
	SRPC_SYNC_METHOD = 0
	SRPC_CALL_METHOD = 1
)

type ReqHeader struct {
	ReqID    uint64
	MethodID uint32
	Body     []byte
}

type RspHeader struct {
	ReqID uint64
	ErrNo uint32
	Body  []byte
}

type MethodAll struct {
	Method []MethodInfo
}

type MethodValue struct {
	FuncValue reflect.Value
	FuncType  reflect.Type
	ReqType   reflect.Type
	RspType   reflect.Type
}

type Server struct {
	functable *Method
	funcvalue map[uint32]MethodValue
	rwlock    sync.RWMutex
	lis       *comm.Listen
}

func NewServer(addr string) *Server {
	s := new(Server)
	s.lis = comm.NewListen(addr)
	if s.lis == nil {
		return nil
	}
	s.funcvalue = make(map[uint32]MethodValue, 100)
	s.functable = NewMethod()
	return s
}

func (s *Server) RegMethod(pthis interface{}) {

	//创建反射变量，注意这里需要传入ruTest变量的地址；
	//不传入地址就只能反射Routers静态定义的方法

	vfun := reflect.ValueOf(pthis)
	vtype := vfun.Type()

	//读取方法数量
	num := vfun.NumMethod()

	log.Println("Method Num:", num)

	//遍历路由器的方法，并将其存入控制器映射变量中
	for i := 0; i < num; i++ {

		var fun MethodValue

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
			log.Printf("function %s (output type %s) failed! \r\n",
				funname, fun.FuncType.Out(0).String())
			continue
		}

		mid, err := s.functable.Add(funname, fun.ReqType.String(), fun.RspType.String())
		if err != nil {
			log.Println(err.Error())
			continue
		}

		s.funcvalue[mid] = fun

		log.Println("Add Method: ",
			funname, fun.ReqType.String(), fun.RspType.String())
	}
}

func (s *Server) CallMethod(req ReqHeader) (rsp RspHeader, err error) {

	funcvalue, b := s.funcvalue[req.MethodID]
	if b == false {
		log.Println("method is exist!", req)
		return
	}

	reqtype := funcvalue.ReqType
	rsptype := funcvalue.RspType

	var parms [2]reflect.Value
	parms[0] = reflect.New(reqtype)
	parms[1] = reflect.New(rsptype)

	err = comm.BinaryDecoder(req.Body, parms[0].Interface())
	if err != nil {
		log.Println(err.Error())
		return
	}
	parms[0] = reflect.Indirect(parms[0])

	output := funcvalue.FuncValue.Call(parms[0:])

	if output[0].Type().Name() != "error" {
		log.Println("return value type invaild!")
		return
	}

	value := output[0].Interface()
	if value != nil {
		rsp.ErrNo = 1
		rsp.ReqID = req.ReqID
		rsp.Body, err = comm.BinaryCoder(value)
	} else {
		rsp.ErrNo = 0
		rsp.ReqID = req.ReqID
		rsp.Body, err = comm.BinaryCoder(reflect.Indirect(parms[1]).Interface())
	}

	if err != nil {
		log.Println(err.Error())
		return
	}

	return
}

func (c *Server) reqMsgProcess(conn *comm.Server, reqid uint32, body []byte) {
	var req ReqHeader

	req.ReqID = comm.GetUint64(body)
	req.MethodID = comm.GetUint32(body[8:])
	req.Body = body[12:]

	//log.Println("recv req: ", req)

	rsp, err := c.CallMethod(req)
	if err != nil {
		log.Println(err.Error())

		StatAdd(1, 0, 1)
		return
	}

	body = make([]byte, 12+len(rsp.Body))
	comm.PutUint64(rsp.ReqID, body)
	comm.PutUint32(rsp.ErrNo, body[8:])
	copy(body[12:], rsp.Body)

	err = conn.SendMsg(SRPC_CALL_METHOD, body)
	if err != nil {

		StatAdd(1, 0, 1)
		return
	}

	StatAdd(1, 1, 0)

	//log.Println("send rsp: ", rsp)
}

func (c *Server) reqMethodProcess(conn *comm.Server, reqid uint32, body []byte) {

	var rspmethod MethodAll

	rspmethod.Method = c.functable.GetBatch()

	body, err := comm.BinaryCoder(rspmethod)
	if err != nil {
		log.Println(err.Error())
		return
	}

	err = conn.SendMsg(SRPC_SYNC_METHOD, body)
	if err != nil {
		return
	}
}

func (s *Server) Start() {

	cpunum := runtime.NumCPU()

	for {

		comm, err := s.lis.Accept()
		if err != nil {
			log.Println(err.Error())
			return
		}

		log.Println("new server instance")

		comm.RegHandler(SRPC_SYNC_METHOD, s.reqMethodProcess)
		comm.RegHandler(SRPC_CALL_METHOD, s.reqMsgProcess)

		go func() {
			comm.Start(cpunum, 1000)
			comm.Wait()

			log.Println("close server instance")
		}()
	}
}
