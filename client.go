package srpc

import (
	"errors"
	"log"
	"reflect"
	"srpc/comm"
	"sync"
	"sync/atomic"
)

type Client struct {
	ReqID uint64
	conn  *comm.Client

	ReqQue chan *Request
	RspQue chan *Rsponse

	functable *Method

	done chan bool

	wait sync.WaitGroup
}

type Request struct {
	msg    ReqHeader
	result *Result
}

type Rsponse struct {
	msg    RspHeader
	result *Result
}

type Result struct {
	Err    error
	Method string
	Req    interface{}
	Rsp    interface{}
	Done   chan *Result
}

func NewClient(addr string) *Client {
	c := new(Client)
	c.conn = comm.NewClient(addr)
	c.ReqQue = make(chan Request, 1000)
	c.RspQue = make(chan Rsponse, 1000)
	c.functable = NewMethod()
	c.done = make(chan bool)
	return c
}

func (c *Client) reqMsgProcess() {
	defer c.wait.Done()

	result := make(map[uint64]*Result, 100000)

	for {

		select {
		case reqmsg := <-c.ReqQue:
			{
				result[reqmsg.msg.ReqID] = reqmsg.result
			}
		case rspmsg := <-c.RspQue:
			{
				temp, b := result[rspmsg.msg.ReqID]
				if b == false {
					log.Println("drop error rsp!", rspmsg)
					continue
				}
				delete(result, rspmsg.msg.ReqID)

				//temp.msg = rspmsg
				//temp.Done <- temp
			}
		}
	}
}

func (c *Client) rspMsgProcess(conn *comm.Client, reqid uint32, body []byte) {

}

func (c *Client) rspMethodProcess(conn *comm.Client, reqid uint32, body []byte) {

	var functable MethodAll
	functable.method = make([]MethodInfo, 0)

	err := comm.DecodePacket(body, &functable)
	if err != nil {
		log.Println(err.Error())
		c.done <- false
		return
	}

	err = c.functable.BatchAdd(functable.method)
	if err != nil {
		log.Println(err.Error())
		c.done <- false
		return
	}

	c.done <- true
}

func (c *Client) Start(num int) error {

	err := c.conn.RegHandler(SRPC_SYNC_METHOD, c.rspMethodProcess)
	if err != nil {
		return err
	}

	err = c.conn.RegHandler(SRPC_CALL_METHOD, c.rspMsgProcess)
	if err != nil {
		return err
	}

	err = c.conn.Start(num)
	if err != nil {
		return err
	}

	err = c.conn.SendMsg(0, nil)
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

func (c *Client) Call(method string, req interface{}, rsp interface{}) error {

	reqvalue := reflect.ValueOf(req)
	rspvalue := reflect.ValueOf(rsp)

	if reqvalue.Kind() == reflect.Ptr {
		return errors.New("parm req is ptr type!")
	}

	if rspvalue.Kind() != reflect.Ptr {
		return errors.New("parm rsp is'nt ptr type!")
	}
	rspvalue = reflect.Indirect(rspvalue)

	funcinfo, err := c.functable.GetByName(method)
	if err != nil {
		return
	}

	if funcinfo.Name != method ||
		funcinfo.Req != reqvalue.Type().String() ||
		funcinfo.Rsp != rspvalue.Type().String() {

		return errors.New("method type not match!", funcinfo)
	}

	reqblock := new(Request)
	reqblock.msg.ReqID = atomic.AddUint64(&c.ReqID, 1)
	reqblock.msg.MethodID = funcinfo.ID
	reqblock.msg.Body, err = comm.CodePacket(req)
	if err != nil {
		log.Println(err.Error())
		return err
	}
	reqblock.result = new(Result)
	reqblock.result.Done = make(chan *Result)
	reqblock.result.Req = req
	reqblock.result.Rsp = rsp
	reqblock.result.Method = method

	c.ReqQue <- reqblock

	result := <-reqblock.result.Done

	var reqblock RequestBlock
	var rspblock RsponseBlock

	reqblock.Method = method
	reqblock.MsgId = n.MsgId
	reqblock.Parms[0] = requestValue.Type().String()
	reqblock.Parms[1] = rsponseValue.Type().String()

	reqblock.Body, err = CodePacket(req)

	// 序列化请求报文
	newbuf, err := CodePacket(reqblock)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	err = FullyWrite(n.socket, newbuf)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	// 反序列化报文
	err = comm.DecodePacket(buf[0:cnt], &rspblock)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	//log.Println(rspblock)

	// 校验请求的序号是否一致
	if rspblock.MsgId != reqblock.MsgId {
		err = errors.New("recv a bad packet ")
		log.Println(err.Error())
		return err
	}

	// 反序列化报文
	err = comm.DecodePacket(rspblock.Body, rsp)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	if rspblock.Result != "success" {
		return errors.New(rspblock.Result)
	}

	return nil
}

func (c *Client) CallAsync(method string, req interface{}, rsp interface{}, done chan *Result) (chan *Result, error) {

	var err error
	requestValue := reflect.ValueOf(req)
	rsponseValue := reflect.ValueOf(rsp)

	// 校验参数合法性，req必须是非指针类型，rsp必须是指针类型
	if rsponseValue.Kind() != reflect.Ptr {
		return nil, errors.New("parm rsp is'nt ptr type!")
	}

	if requestValue.Kind() == reflect.Ptr {
		return nil, errors.New("parm req is ptr type!")
	}

	rsponseValue = reflect.Indirect(rsponseValue)

	ReqID := atomic.AddUint64(&c.ReqID, 1)

	var reqblock RequestBlock

	reqblock.Method = method
	reqblock.MsgId = n.MsgId
	reqblock.Parms[0] = requestValue.Type().String()
	reqblock.Parms[1] = rsponseValue.Type().String()
	reqblock.Body, err = CodePacket(req)
	if err != nil {
		Log(err.Error())
		return err
	}

	var result Result

	result.Err = nil
	result.Req = req
	result.Rsp = rsp
	result.MsgId = reqblock.MsgId

	n.lock.Lock()
	n.Sended[reqblock.MsgId] = result
	n.lock.Unlock()

	//log.Println(reqblock)

	n.ReqBlock <- reqblock

	return nil
}

func (c *Client) Stop() {
	c.conn.Stop()
	c.wait.Wait()
}
