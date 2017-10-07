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
	msg RspHeader
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

	resultTable := make(map[uint64]*Result, 10000)

	for {

		select {
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
		case <-c.done:
			{
				log.Println("msg proc shutdown!")
				return
			}
		}
	}
}

func (c *Client) rspMsgProcess(conn *comm.Client, reqid uint32, body []byte) {

	rsp := new(Rsponse)
	rsp.msg.ReqID = comm.GetUint64(body)
	rsp.msg.ErrNo = comm.GetUint32(body[8:])
	rsp.msg.Body = body[12:]

	c.RspQue <- rsp
}

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

func (c *Client) Call(method string, req interface{}, rsp interface{}) error {

	done := make(chan *Result, 10)

	c.CallAsync(method, req, rsp, done)
	result := <-done

	if result.Err != nil {
		return result.Err
	}

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

func (c *Client) Stop() {

	c.conn.Stop()
	c.conn.Wait()

	c.done <- true

	c.wait.Wait()
}
