package srpc

import (
	"errors"
	"log"
	"net"
	"reflect"
	"runtime/debug"
	"time"
)

func Log(v ...interface{}) {
	log.Println(v)
	log.Println(string(debug.Stack()))
}

type Client struct {
	Addr   string
	MsgId  uint64
	socket net.Conn
}

func NewClient(network, addr string) *Client {
	var client = Client{Addr: addr}
	client.MsgId = uint64(time.Now().Nanosecond())

	// 创建udp协议的socket服务
	socket, err := net.Dial(network, addr)
	if err != nil {
		return nil
	}

	client.socket = socket

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

	reqblock.MsgType = 0
	reqblock.Method = method
	reqblock.MsgId = n.MsgId
	reqblock.Parms[0] = requestValue.Type().String()
	reqblock.Parms[1] = rsponseValue.Type().String()
	reqblock.Body, err = CodePacket(req)
	if err != nil {
		Log(err.Error())
		return err
	}

	socket := n.socket

	// 设置 read/write 超时时间
	err = socket.SetDeadline(time.Now().Add(2 * time.Second))
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

	// 发送到服务端
	_, err = socket.Write(newbuf)
	if err != nil {
		Log(err.Error())
		return err
	}

	var buf [4096]byte

	// 获取服务端应答报文
	cnt, err := socket.Read(buf[0:])
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

func (n *Client) Delete() {
	n.socket.Close()
}
