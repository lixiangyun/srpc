package srpc

import (
	"log"
	"reflect"
	"runtime"
	"sync"

	"github.com/lixiangyun/comm"
)

// 定义请求ID
const (
	SRPC_SYNC_METHOD = 0 // 客户端同步服务端方法信息的请求ID
	SRPC_CALL_METHOD = 1 // 客户端调用服务端方法的请求ID
)

// 请求报文结构
type ReqHeader struct {
	ReqID    uint64 // 请求ID
	MethodID uint32 // 请求方法ID
	Body     []byte // 内容
}

// 应答报文结构
type RspHeader struct {
	ReqID uint64 // 请求ID
	ErrNo uint32 // 错误码
	Body  []byte // 内容
}

// 包含全部的方法信息，用于客户端同步服务端的方法信息；
type MethodAll struct {
	Method []MethodInfo
}

// 方法的反射管理结构
type MethodValue struct {
	FuncValue reflect.Value // 方法的反射
	FuncType  reflect.Type  // 方法的类型反射
	ReqType   reflect.Type  // 请求参数类型的反射
	RspType   reflect.Type  // 应答参数类型的反射
}

// 服务端对象的结构
type Server struct {
	functable *Method                // 方法的管理结构
	funcvalue map[uint32]MethodValue // 方法的反射
	rwlock    sync.RWMutex           // 读写锁
	lis       *comm.Listen           // 通信通道对象
}

// 申请一个RPC服务端对象
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

// 使能tls认证&加密
func (s *Server) TlsEnable(ca, cert, key string) error {
	err := s.lis.TlsEnable(ca, cert, key)
	if err != nil {
		log.Println("RPC TlsEnable: ", err.Error())
	}
	return err
}

// 注册方法，需要传入对象的指针。并且对象需要有相应的处理方法；
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

/* 调用服务端提供的方法，根据方法ID获取到相应的方法，并且构造参数，
 * 传递给方法执行，将执行的结果发送给客户端。
 */
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

// rpc调用请求的服务端处理函数；
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

// 客户端请求查询服务端提供的方法信息；
func (c *Server) reqMethodProcess(conn *comm.Server, reqid uint32, body []byte) {

	var rspmethod MethodAll

	// 获取服务端提供的全部的方法
	rspmethod.Method = c.functable.GetBatch()

	// 编码方法信息成二进制body内容
	body, err := comm.BinaryCoder(rspmethod)
	if err != nil {
		log.Println(err.Error())
		return
	}

	// 发送给客户端处理
	err = conn.SendMsg(SRPC_SYNC_METHOD, body)
	if err != nil {
		return
	}
}

//服务端RPC启动函数，启动tcp端口监听，并且创建服务实例；
func (s *Server) Start() {

	/* 获取本地环境的cpu数量，用于注册给服务端实例
	 * 处理请求的调度任务，提升请求的处理并发量；
	 */
	cpunum := runtime.NumCPU()

	for {

		// 监听服务端口
		comm, err := s.lis.Accept()
		if err != nil {
			log.Println(err.Error())
			return
		}

		log.Println("new server instance")

		// 注册请求ID对应的处理函数
		comm.RegHandler(SRPC_SYNC_METHOD, s.reqMethodProcess)
		comm.RegHandler(SRPC_CALL_METHOD, s.reqMsgProcess)

		// 启动服务实例，并且等待服务实例销毁；
		go func() {
			comm.Start(cpunum, 1000)
			comm.Wait()

			log.Println("close server instance")
		}()
	}
}
