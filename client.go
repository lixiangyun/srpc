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
	c.ReqQue = make(chan *Request, 1000)
	c.RspQue = make(chan *Rsponse, 1000)
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

	return nil
}

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

	defer func() {
		done <- result
	}()

	reqvalue := reflect.ValueOf(req)
	rspvalue := reflect.ValueOf(rsp)

	if reqvalue.Kind() == reflect.Ptr {
		result.Err = errors.New("parm req is ptr type!")
		return done
	}

	if rspvalue.Kind() != reflect.Ptr {
		result.Err = errors.New("parm rsp is'nt ptr type!")
		return done
	}
	rspvalue = reflect.Indirect(rspvalue)

	funcinfo, err := c.functable.GetByName(method)
	if err != nil {
		result.Err = err
		return done
	}

	if funcinfo.Name != method ||
		funcinfo.Req != reqvalue.Type().String() ||
		funcinfo.Rsp != rspvalue.Type().String() {
		result.Err = errors.New("method type not match!")
		return done
	}

	reqblock := new(Request)
	reqblock.msg.ReqID = atomic.AddUint64(&c.ReqID, 1)
	reqblock.msg.MethodID = funcinfo.ID
	reqblock.msg.Body, err = comm.CodePacket(req)
	if err != nil {
		result.Err = err
		return done
	}
	reqblock.result = result

	c.ReqQue <- reqblock

	return done
}

func (c *Client) Stop() {
	c.conn.Stop()
	c.wait.Wait()
}
