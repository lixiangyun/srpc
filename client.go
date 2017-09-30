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

	ReqMsg chan RequestMsg
	RspMsg chan RspHeader

	methodByName map[string]*MethodInfo
	methodById   map[uint32]*MethodInfo

	done chan bool

	lock sync.Mutex
	wait sync.WaitGroup
}

type RequestMsg struct {
	msghead ReqHeader
	result  Result
}

type Result struct {
	Err    error
	Method string
	Req    interface{}
	Rsp    interface{}

	Done chan *Result

	rspmsg *RspHeader
	reqmsg *ReqHeader
}

func NewClient(addr string) *Client {

	c := new(Client)
	c.conn = comm.NewClient(addr)

	c.ReqMsg = make(chan RequestMsg, 1000)
	c.RspMsg = make(chan RspHeader, 1000)

	c.methodByName = make(map[string]*MethodInfo, 0)
	c.methodById = make(map[uint32]*MethodInfo, 0)

	c.done = make(chan bool)

	return c
}

func (c *Client) reqMsgProcess() {
	defer c.wait.Done()

	result := make(map[uint64]*Result, 100000)

	for {

		select {
		case reqmsg := <-c.ReqMsg:
			{
				result[reqmsg.msghead.ReqID] = &reqmsg.result
			}
		case rspmsg := <-c.RspMsg:
			{
				temp, b := result[rspmsg.ReqID]
				if b == false {
					log.Println("drop error rsp!", rspmsg)
					continue
				}
				delete(result, rspmsg.ReqID)

				temp.msg = rspmsg
				temp.Done <- temp
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

	c.methodByName = make(map[string]*MethodInfo, functable.size)
	c.methodById = make(map[uint32]*MethodInfo, functable.size)

	for idx, vfun := range functable.method {
		newfun := &(vfun)
		c.methodByName[vfun.Name] = newfun
		c.methodById[vfun.ID] = newfun

		log.Println("sync method: ", *newfun)
	}

	c.done <- true
}

func (c *Client) Start(num int) error {

	err := c.conn.RegHandler(0, c.rspMethodProcess)
	if err != nil {
		return err
	}

	err = c.conn.RegHandler(1, c.rspMsgProcess)
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

	result := <-c.done
	if result == false {
		return errors.New("sync method failed!")
	}

	c.wait.Add(1)
	go c.reqMsgProcess()

	return nil
}

func (c *Client) Call(method string, req interface{}, rsp interface{}) error {

	var err error

	requestValue := reflect.ValueOf(req)
	rsponseValue := reflect.ValueOf(rsp)

	// 校验参数合法性，req必须是非指针类型，rsp必须是指针类型
	if rsponseValue.Kind() != reflect.Ptr {
		return errors.New("parm rsp is'nt ptr type!")
	}

	if requestValue.Kind() == reflect.Ptr {
		return errors.New("parm req is ptr type!")
	}

	rsponseValue = reflect.Indirect(rsponseValue)

	ReqID := atomic.AddUint64(&c.ReqID, 1)

	var reqblock RequestBlock
	var rspblock RsponseBlock

	reqblock.Method = method
	reqblock.MsgId = n.MsgId
	reqblock.Parms[0] = requestValue.Type().String()
	reqblock.Parms[1] = rsponseValue.Type().String()
	reqblock.Body, err = CodePacket(req)
	if err != nil {
		Log(err.Error())
		return err
	}

	// 序列化请求报文
	newbuf, err := CodePacket(reqblock)
	if err != nil {
		Log(err.Error())
		return err
	}

	err = FullyWrite(n.socket, newbuf)
	if err != nil {
		Log(err.Error())
		return err
	}

	// 反序列化报文
	err = DecodePacket(buf[0:cnt], &rspblock)
	if err != nil {
		Log(err.Error())
		return err
	}

	//log.Println(rspblock)

	// 校验请求的序号是否一致
	if rspblock.MsgId != reqblock.MsgId {
		err = errors.New("recv a bad packet ")
		Log(err.Error())
		return err
	}

	// 反序列化报文
	err = DecodePacket(rspblock.Body, rsp)
	if err != nil {
		Log(err.Error())
		return err
	}

	if rspblock.Result != "success" {
		return errors.New(rspblock.Result)
	}

	return nil
}

func (n *Client) GetRelay() chan Result {
	return n.Reply
}

func (n *Client) SendProccess() {

	defer n.wait.Done()

	reqarray := make([]RequestBlock, 1001)

	for {

		index := 0

		reqblock := <-n.ReqBlock
		reqarray[index] = reqblock
		index++

		num := len(n.ReqBlock)
		if num > 1000 {
			num = 1000
		}

		for i := 0; i < num; i++ {
			reqblock := <-n.ReqBlock
			reqarray[index] = reqblock
			index++
		}

		err := SendRequestBlock(n.socket, reqarray[0:index])
		if err != nil {
			log.Println(err.Error())
			return
		}
	}
}

func (n *Client) AsyncProccess() {

	defer n.wait.Done()

	var buf [4096]byte
	var tmpbuf [4096]byte
	length := 0

	for {

		// 获取服务端应答报文
		cnt, err := n.socket.Read(buf[length:])
		if err != nil {
			log.Println(err.Error())
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

			var rspblock RsponseBlock

			// 反序列化报文
			err = DecodePacket(tmpbuf[0:index], &rspblock)
			if err != nil {
				Log(err.Error())
				continue
			}

			n.lock.Lock()
			result, b := n.Sended[rspblock.MsgId]
			if b == false {
				n.lock.Unlock()
				Log("can not found this msgid!", rspblock.MsgId)
				continue
			}
			delete(n.Sended, rspblock.MsgId)
			n.lock.Unlock()

			// 反序列化报文
			err = DecodePacket(rspblock.Body, result.Rsp)
			if err != nil {
				Log(err.Error())
				continue
			}

			n.Reply <- result
		}
	}
}

func (n *Client) CallAsync(method string, req interface{}, rsp interface{}) error {

	var err error
	requestValue := reflect.ValueOf(req)
	rsponseValue := reflect.ValueOf(rsp)

	// 校验参数合法性，req必须是非指针类型，rsp必须是指针类型
	if rsponseValue.Kind() != reflect.Ptr {
		return errors.New("parm rsp is'nt ptr type!")
	}

	if requestValue.Kind() == reflect.Ptr {
		return errors.New("parm req is ptr type!")
	}

	rsponseValue = reflect.Indirect(rsponseValue)

	n.MsgId++

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
