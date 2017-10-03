package comm

import (
	"errors"
	"log"
	"net"
	"sync"
)

type ClientHandler func(c *Client, reqid uint32, body []byte)

type Client struct {
	addr    string
	conn    *connect
	handler map[uint32]ClientHandler
	wait    sync.WaitGroup
}

func NewClient(addr string) *Client {
	c := Client{addr: addr}
	c.handler = make(map[uint32]ClientHandler, 10)
	return &c
}

func (s *Client) RegHandler(reqid uint32, fun ClientHandler) error {
	_, b := s.handler[reqid]
	if b == true {
		return errors.New("channel has been register!")
	}
	s.handler[reqid] = fun
	return nil
}

func msgprocess_client(c *Client) {

	defer c.wait.Done()

	for {
		msg, b := <-c.conn.recvbuf
		if b == false {
			return
		}

		fun, b := c.handler[msg.ReqID]
		if b == false {
			log.Println("can not found [", msg.ReqID, "] handler!")
		} else {
			fun(c, msg.ReqID, msg.Body)
		}
	}
}

func (c *Client) Start(num int) error {

	conn, err := net.Dial("tcp", c.addr)
	if err != nil {
		return err
	}

	c.conn = NewConnect(conn, 1000)

	c.wait.Add(num)
	for i := 0; i < num; i++ {
		go msgprocess_client(c)
	}

	return nil
}

func (c *Client) Stop() {
	c.conn.WaitClose()
	c.wait.Done()
	log.Println("client close.")
}

func (c *Client) SendMsg(reqid uint32, body []byte) error {
	var msg Header

	msg.ReqID = reqid
	msg.Body = make([]byte, len(body))
	copy(msg.Body, body)
	c.conn.sendbuf <- msg

	return nil
}
