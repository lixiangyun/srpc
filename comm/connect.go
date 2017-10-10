package comm

import (
	"errors"
	"log"
	"net"
	"sync"
)

const (
	MAX_BUF_SIZE = 128 * 1024 // 缓冲区大小(单位：byte)
	MAGIC_FLAG   = 0x98b7f30a // 校验魔术字
	MSG_HEAD_LEN = 3 * 4      // 消息头长度
)

// 消息头
type Header struct {
	ReqID uint32 // 请求ID
	Body  []byte // 传输内容
}

// 内部传输的报文头
type msgHeader struct {
	Flag  uint32 // 魔术字
	ReqID uint32 // 请求ID
	Size  uint32 // 内容长度
	Body  []byte // 传输的内容
}

// 链路管理的资源结构
type connect struct {
	bexit    bool
	exit     chan bool
	conn     net.Conn       // 链路结构
	wait     sync.WaitGroup // 同步等待退出
	taskexit chan bool      // 退出通道
	SendBuf  chan Header    // 发送缓冲队列
	RecvBuf  chan Header    // 接收缓冲队列
}

// 申请链路操作资源
func NewConnect(conn net.Conn, buflen int) *connect {

	c := new(connect)

	c.conn = conn
	c.SendBuf = make(chan Header, buflen)
	c.RecvBuf = make(chan Header, buflen)
	c.taskexit = make(chan bool, 10)
	c.exit = make(chan bool, 10)

	c.wait.Add(2)
	go c.sendtask()
	go c.recvtask()

	return c
}

// 等待链路资源销毁
func (c *connect) Wait() {
	<-c.exit
	c.bexit = true
	c.taskexit <- true
	c.conn.Close()
	c.wait.Wait()
}

// 主动发起资源销毁
func (c *connect) Close() {
	c.exit <- true
}

// 序列化报文头
func coderMsg(msg Header) []byte {

	size := len(msg.Body)
	tmpbuf := make([]byte, MSG_HEAD_LEN+size)

	PutUint32(MAGIC_FLAG, tmpbuf[0:])
	PutUint32(msg.ReqID, tmpbuf[4:])
	PutUint32(uint32(size), tmpbuf[8:])
	copy(tmpbuf[12:], msg.Body)

	return tmpbuf
}

// 构造消息发送至消息队列
func decoderMsg(regid uint32, body []byte) Header {

	var tempmsg Header
	tempmsg.ReqID = regid
	tempmsg.Body = make([]byte, len(body))
	copy(tempmsg.Body, body)

	return tempmsg
}

// 发送调度协成
func (c *connect) sendtask() {

	defer c.wait.Done()
	var buf [MAX_BUF_SIZE]byte

	for {

		var buflen int
		var msg Header

		// 监听消息发送缓存队列
		select {
		case msg = <-c.SendBuf:
		case <-c.taskexit:
			{
				return
			}
		}

		tmpbuf := coderMsg(msg)
		tmpbuflen := len(tmpbuf)

		if tmpbuflen >= MAX_BUF_SIZE/2 {
			err := fullywrite(c.conn, tmpbuf[0:])
			if err != nil {
				log.Println(err.Error())
				c.Close()
				return
			}
		} else {
			copy(buf[0:tmpbuflen], tmpbuf[0:])
			buflen = tmpbuflen
		}

		// 从消息缓存队列中批量获取消息，并且合并消息一次发送。
		chanlen := len(c.SendBuf)

		for i := 0; i < chanlen; i++ {

			msg = <-c.SendBuf

			tmpbuf = coderMsg(msg)
			tmpbuflen = len(tmpbuf)

			copy(buf[buflen:buflen+tmpbuflen], tmpbuf[0:])
			buflen += tmpbuflen

			if buflen >= MAX_BUF_SIZE/2 {
				err := fullywrite(c.conn, buf[0:buflen])
				if err != nil {
					log.Println(err.Error())
					c.Close()
					return
				}
				buflen = 0
			}
		}

		if buflen > 0 {
			err := fullywrite(c.conn, buf[0:buflen])
			if err != nil {
				log.Println(err.Error())
				c.Close()
				return
			}
		}
	}
}

// 接收调度协成
func (c *connect) recvtask() {

	var buf [MAX_BUF_SIZE]byte
	var totallen int

	defer c.wait.Done()

	for c.bexit != true {

		var lastindex int

		// 从socket读取数据
		recvnum, err := c.conn.Read(buf[totallen:])
		if err != nil {
			log.Println(err.Error())
			c.Close()
			return
		}

		totallen += recvnum

		for {

			if lastindex+MSG_HEAD_LEN > totallen {
				copy(buf[0:totallen-lastindex], buf[lastindex:totallen])
				totallen = totallen - lastindex
				break
			}

			// 反序列化报文内容
			Flag := GetUint32(buf[lastindex:])
			ReqID := GetUint32(buf[lastindex+4:])
			Size := GetUint32(buf[lastindex+8:])

			bodybegin := lastindex + MSG_HEAD_LEN
			bodyend := bodybegin + int(Size)

			// 校验消息头魔术字
			if Flag != MAGIC_FLAG {

				log.Println("BadMsg:")
				log.Println("TotalLen:", totallen)
				log.Println("BodyBegin:", bodybegin, " bodyend:", bodyend)
				log.Println("Body:", buf[lastindex:bodyend])
				log.Println("BodyFull:", buf[0:totallen])

				c.Close()
				return
			}

			if bodyend > totallen {
				copy(buf[0:totallen-lastindex], buf[lastindex:totallen])
				totallen = totallen - lastindex
				break
			}

			c.RecvBuf <- decoderMsg(ReqID, buf[bodybegin:bodyend])

			lastindex = bodyend
		}
	}
}

// 发送消息
func (c *connect) SendMsg(msg Header) error {
	if c.bexit == true {
		return errors.New("connect closed.")
	} else {
		c.SendBuf <- msg
		return nil
	}
}

// 发送封装的接口
func fullywrite(conn net.Conn, buf []byte) error {
	totallen := len(buf)
	sendcnt := 0

	for {
		cnt, err := conn.Write(buf[sendcnt:])
		if err != nil {
			return err
		}
		if cnt+sendcnt >= totallen {
			return nil
		}
		sendcnt += cnt
	}
}
