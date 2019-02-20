package srpc

import (
	"errors"
	"log"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/lixiangyun/comm"
)

type Client struct {
	ReqID uint64       // 请求ID
	conn  *comm.Client // 通信结构信息

	ReqQue chan *Request // 请求消息缓存队列
	RspQue chan *Rsponse // 应答消息缓存队列

	functable *Method        // 符号表信息
	done      chan bool      // 结束通道
	wait      sync.WaitGroup // 等待资源销毁同步等待
}

type Request struct {
	msg    ReqHeader // 请求消息头
	result *Result   // 请求上下文信息
}

type Rsponse struct {
	msg RspHeader // 消息头
}

type Result struct {
	Err    error        // 错误信息
	Method string       // 请求的方法名称
	Req    interface{}  // 请求的参数
	Rsp    interface{}  // 应答的参数
	Done   chan *Result // 请求等待通道
}

// 申请一个srpc客户端资源
func NewClient(addr string) *Client {
	c := new(Client)
	c.conn = comm.NewClient(addr)
	c.ReqQue = make(chan *Request, 1000)
	c.RspQue = make(chan *Rsponse, 1000)
	c.functable = NewMethod()
	c.done = make(chan bool)
	return c
}

// 使能tls认证&加密
func (c *Client) TlsEnable(ca, cert, key string) error {
	err := c.conn.TlsEnable(ca, cert, key)
	if err != nil {
		log.Println("RPC TlsEnable: ", err.Error())
	}
	return err
}

// 请求&应答消息缓存处理任务
func (c *Client) reqMsgProcess() {
	defer c.wait.Done()

	resultTable := make(map[uint64]*Result, 10000)

	for {

		select {
		// 读取请求消息缓存队列
		case reqmsg := <-c.ReqQue:
			{
				//log.Println("ReqMsg: ", reqmsg.msg)

				_, b := resultTable[reqmsg.msg.ReqID]
				if b == true {
					log.Println("double reqID !", reqmsg)
					continue
				}
				resultTable[reqmsg.msg.ReqID] = reqmsg.result

				body := make([]byte, 12+len(reqmsg.msg.Body))
				comm.PutUint64(reqmsg.msg.ReqID, body)
				comm.PutUint32(reqmsg.msg.MethodID, body[8:])
				copy(body[12:], reqmsg.msg.Body)

				c.conn.SendMsg(SRPC_CALL_METHOD, body)
			}
			// 读取应答消息缓存队列
		case rspmsg := <-c.RspQue:
			{
				var err error

				//log.Println("RspMsg: ", rspmsg.msg)

				result, b := resultTable[rspmsg.msg.ReqID]
				if b == false {
					log.Println("drop error rsp!", rspmsg)
					continue
				}
				delete(resultTable, rspmsg.msg.ReqID)

				if rspmsg.msg.ErrNo == 0 {
					err = comm.BinaryDecoder(rspmsg.msg.Body, result.Rsp)
				} else {
					err = comm.BinaryDecoder(rspmsg.msg.Body, &result.Err)
				}

				if err != nil {
					log.Println(err.Error())
					continue
				}

				result.Done <- result
			}
			// 监听任务退出
		case <-c.done:
			{
				log.Println("msg proc shutdown!")
				return
			}
		}
	}
}

// 应答消息接收函数，将通道的消息反序列化，并且发送至应答缓存队列。
func (c *Client) rspMsgProcess(conn *comm.Client, reqid uint32, body []byte) {

	rsp := new(Rsponse)
	rsp.msg.ReqID = comm.GetUint64(body)
	rsp.msg.ErrNo = comm.GetUint32(body[8:])
	rsp.msg.Body = body[12:]

	c.RspQue <- rsp
}

// 服务端方法同步处理，将服务端的调用方法，同步到客户端本地，提供快速查询匹配，提升性能。
func (c *Client) rspMethodProcess(conn *comm.Client, reqid uint32, body []byte) {

	var functable MethodAll
	functable.Method = make([]MethodInfo, 0)

	err := comm.BinaryDecoder(body, &functable)
	if err != nil {
		log.Println(err.Error())
		c.done <- false
		return
	}

	err = c.functable.BatchAdd(functable.Method)
	if err != nil {
		log.Println(err.Error())
		c.done <- false
		return
	}

	for _, vfun := range functable.Method {
		log.Println("Sync Method: ", vfun)
	}

	c.done <- true
}

// 客户端启动函数，用于启动客户端通信和调度任务
func (c *Client) Start() error {

	err := c.conn.RegHandler(SRPC_SYNC_METHOD, c.rspMethodProcess)
	if err != nil {
		return err
	}

	err = c.conn.RegHandler(SRPC_CALL_METHOD, c.rspMsgProcess)
	if err != nil {
		return err
	}

	err = c.conn.Start(1, 1000)
	if err != nil {
		return err
	}

	err = c.conn.SendMsg(SRPC_SYNC_METHOD, nil)
	if err != nil {
		return err
	}

	b := <-c.done
	if b == false {
		return errors.New("sync method failed!")
	}

	c.wait.Add(1)
	go c.reqMsgProcess()

	return nil
}

// 客户端提供的RPC同步调用方法
func (c *Client) Call(method string, req interface{}, rsp interface{}) error {

	done := make(chan *Result, 10)

	c.CallAsync(method, req, rsp, done)
	result := <-done

	if result.Err != nil {
		return result.Err
	}

	return nil
}

// 客户端提供的RPC异步调用方法，需要用户传入等待通道，以便在接收到应答之后，唤醒并且返回处理结果；
func (c *Client) CallAsync(method string, req interface{}, rsp interface{}, done chan *Result) chan *Result {

	if done == nil {
		log.Println("input error!")
		return nil
	}

	result := new(Result)
	result.Method = method
	result.Done = done
	result.Req = req
	result.Rsp = rsp

	reqvalue := reflect.ValueOf(req)
	rspvalue := reflect.ValueOf(rsp)

	if reqvalue.Kind() == reflect.Ptr {
		result.Err = errors.New("parm req is ptr type!")
		done <- result
		return done
	}

	if rspvalue.Kind() != reflect.Ptr {
		result.Err = errors.New("parm rsp is'nt ptr type!")
		done <- result
		return done
	}
	rspvalue = reflect.Indirect(rspvalue)

	funcinfo, err := c.functable.GetByName(method)
	if err != nil {
		result.Err = err
		done <- result
		return done
	}

	if funcinfo.Name != method ||
		funcinfo.Req != reqvalue.Type().String() ||
		funcinfo.Rsp != rspvalue.Type().String() {
		result.Err = errors.New("method type not match!")

		log.Println("funcinfo : ", funcinfo)
		log.Println("method : ", method, reqvalue.Type().String(), rspvalue.Type().String())

		done <- result
		return done
	}

	reqblock := new(Request)
	reqblock.msg.ReqID = atomic.AddUint64(&c.ReqID, 1)
	reqblock.msg.MethodID = funcinfo.ID
	reqblock.msg.Body, err = comm.BinaryCoder(req)
	if err != nil {
		result.Err = err
		done <- result

		return done
	}

	reqblock.result = result
	c.ReqQue <- reqblock

	return done
}

// 客户端rpc服务停止函数，停止并且释放客户端资源；
func (c *Client) Stop() {

	c.conn.Stop()
	c.conn.Wait()

	c.done <- true

	c.wait.Wait()
}
