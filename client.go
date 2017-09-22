package srpc

import (
	"errors"
	"log"
	"net"
	"reflect"
	"runtime/debug"
	"sync"
)

func Log(v ...interface{}) {
	log.Println(v)
	log.Println(string(debug.Stack()))
}

type Client struct {
	Addr     string
	MsgId    uint64
	socket   net.Conn
	Reply    chan Result
	Sended   map[uint64]Result
	ReqBlock chan RequestBlock

	lock sync.Mutex
	wait sync.WaitGroup
}

type Result struct {
	Err   error
	Req   interface{}
	Rsp   interface{}
	MsgId uint64
}

func NewClient(addr string) *Client {

	var client = Client{Addr: addr}

	// 创建udp协议的socket服务
	socket, err := net.Dial("tcp", addr)
	if err != nil {
		log.Println(err.Error())
		return nil
	}

	client.ReqBlock = make(chan RequestBlock, 1000)
	client.Reply = make(chan Result, 1000)
	client.Sended = make(map[uint64]Result, 1000)
	client.socket = socket

	client.wait.Add(2)
	go client.AsyncProccess()
	go client.SendProccess()

	return &client
}

func (n *Client) Call(method string, req interface{}, rsp interface{}) error {

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

	//log.Println(reqblock)

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

	var buf [4096]byte

	// 获取服务端应答报文
	cnt, err := n.socket.Read(buf[0:])
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

	reqarray := make([]RequestBlock, 100)

	for {

		index := 0

		reqblock := <-n.ReqBlock
		reqarray[index] = reqblock
		index++

		num := len(n.ReqBlock)
		if num > 50 {
			num = 50
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

func (n *Client) Delete() {
	n.socket.Close()
	//close(n.Reply)
	n.wait.Wait()
}
