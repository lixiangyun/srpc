package comm

import (
	"errors"
	"log"
	"net"
	"sync"
)

// 客户端消息处理函数类型
type ClientHandler func(c *Client, reqid uint32, body []byte)

// 客户端实例的数据结构
type Client struct {
	taskexit chan bool
	tasknum  int

	addr    string
	conn    *connect
	handler map[uint32]ClientHandler
	wait    sync.WaitGroup
}

// 申请客户端实例
func NewClient(addr string) *Client {
	c := Client{addr: addr}
	c.handler = make(map[uint32]ClientHandler, 100)
	c.taskexit = make(chan bool, 10)
	return &c
}

// 注册客户端消息处理函数
func (s *Client) RegHandler(reqid uint32, fun ClientHandler) error {
	_, b := s.handler[reqid]
	if b == true {
		return errors.New("channel has been register!")
	}
	s.handler[reqid] = fun
	return nil
}

// 客户端消息处理任务
func msgprocess_client(c *Client) {

	defer c.wait.Done()

	for {
		var msg Header

		select {
		case msg = <-c.conn.RecvBuf:
		case <-c.taskexit:
			{
				return
			}
		}

		fun, b := c.handler[msg.ReqID]
		if b == false {
			log.Println("can not found [", msg.ReqID, "] handler!")
		} else {
			fun(c, msg.ReqID, msg.Body)
		}
	}
}

// 启动客户端处理
func (c *Client) Start(num, buflen int) error {

	conn, err := net.Dial("tcp", c.addr)
	if err != nil {
		return err
	}

	c.conn = NewConnect(conn, buflen)
	c.tasknum = num
	c.wait.Add(num)
	for i := 0; i < num; i++ {
		go msgprocess_client(c)
	}

	return nil
}

// 主动发起资源销毁
func (c *Client) Stop() {
	c.conn.Close()
}

// 等待client端资源销毁
func (c *Client) Wait() {
	c.conn.Wait()
	for i := 0; i < c.tasknum; i++ {
		c.taskexit <- true
	}
	c.wait.Wait()
}

// 发送消息结构
func (c *Client) SendMsg(reqid uint32, body []byte) error {
	var msg Header

	msg.ReqID = reqid
	msg.Body = make([]byte, len(body))
	copy(msg.Body, body)

	return c.conn.SendMsg(msg)
}
