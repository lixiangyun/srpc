package comm

import (
	"errors"
	"log"
	"net"
	"sync"
)

const (
	MAX_BUF_SIZE = 128 * 1024
	MAGIC_FLAG   = 0x98b7f30a
	MSG_HEAD_LEN = 3 * 4
)

type Header struct {
	ReqID uint32
	Body  []byte
}

type msgHeader struct {
	Flag  uint32
	ReqID uint32
	Size  uint32
	Body  []byte
}

type connect struct {
	conn net.Conn

	wait sync.WaitGroup

	sendbuf chan Header
	recvbuf chan Header
}

func NewConnect(conn net.Conn, buflen int) *connect {

	c := new(connect)

	c.conn = conn
	c.sendbuf = make(chan Header, buflen)
	c.recvbuf = make(chan Header, buflen)

	c.wait.Add(2)

	go c.sendtask()
	go c.recvtask()

	return c
}

func (c *connect) WaitClose() {
	c.conn.Close()
	c.wait.Done()
}

func (c *connect) sendtask() {

	defer c.wait.Done()
	var buf [MAX_BUF_SIZE]byte

	for {

		var buflen int

		msg := <-c.sendbuf
		tmpbuf := make([]byte, MSG_HEAD_LEN)

		PutUint32(MAGIC_FLAG, tmpbuf[0:])
		PutUint32(msg.ReqID, tmpbuf[4:])
		PutUint32(uint32(len(msg.Body)), tmpbuf[8:])

		tmpbuf = append(tmpbuf, msg.Body...)
		tmpbuflen := len(tmpbuf)

		if tmpbuflen >= MAX_BUF_SIZE/2 {
			err := fullywrite(c.conn, tmpbuf[0:])
			if err != nil {
				log.Println(err.Error())
				return
			}
		} else {
			copy(buf[0:tmpbuflen], tmpbuf[0:])
			buflen = tmpbuflen
		}

		chanlen := len(c.sendbuf)

		for i := 0; i < chanlen; i++ {

			msg = <-c.sendbuf
			tmpbuf = make([]byte, MSG_HEAD_LEN)

			PutUint32(MAGIC_FLAG, tmpbuf[0:])
			PutUint32(msg.ReqID, tmpbuf[4:])
			PutUint32(uint32(len(msg.Body)), tmpbuf[8:])

			tmpbuf = append(tmpbuf, msg.Body...)
			tmpbuflen = len(tmpbuf)

			copy(buf[buflen:buflen+tmpbuflen], tmpbuf[0:])
			buflen += tmpbuflen

			if buflen >= MAX_BUF_SIZE/2 {
				err := fullywrite(c.conn, buf[0:buflen])
				if err != nil {
					log.Println(err.Error())
					return
				}
				buflen = 0
			}
		}

		if buflen > 0 {
			err := fullywrite(c.conn, buf[0:buflen])
			if err != nil {
				log.Println(err.Error())
				return
			}
		}
	}
}

func (c *connect) recvtask() {

	var buf [MAX_BUF_SIZE]byte
	var totallen int

	defer c.wait.Done()

	for {

		var lastindex int

		recvnum, err := c.conn.Read(buf[totallen:])
		if err != nil {
			log.Println(err.Error())
			return
		}

		totallen += recvnum

		for {

			if lastindex+MSG_HEAD_LEN > totallen {
				copy(buf[0:totallen-lastindex], buf[lastindex:totallen])
				totallen = totallen - lastindex
				break
			}

			var msg msgHeader

			msg.Flag = GetUint32(buf[lastindex:])
			msg.ReqID = GetUint32(buf[lastindex+4:])
			msg.Size = GetUint32(buf[lastindex+8:])

			bodybegin := lastindex + MSG_HEAD_LEN
			bodyend := bodybegin + int(msg.Size)

			if msg.Flag != MAGIC_FLAG {

				log.Println("msghead_0:", msg)
				log.Println("totallen:", totallen)
				log.Println("bodybegin:", bodybegin, " bodyend:", bodyend)
				log.Println("body:", buf[lastindex:bodyend])
				log.Println("bodyFull:", buf[0:totallen])

				log.Println("shutdown client.")

				c.conn.Close()
				return
			}

			if bodyend > totallen {

				copy(buf[0:totallen-lastindex], buf[lastindex:totallen])
				totallen = totallen - lastindex
				break
			}

			c.recvbuf <- Header{ReqID: msg.ReqID, Body: buf[bodybegin:bodyend]}

			lastindex = bodyend
		}
	}
}

func fullywrite(conn net.Conn, buf []byte) error {

	totallen := len(buf)
	sendcnt := 0

	for {

		cnt, err := conn.Write(buf[sendcnt:])
		if err != nil {
			return err
		}

		if cnt <= 0 {
			return errors.New("conn write error!")
		}

		if cnt+sendcnt >= totallen {
			return nil
		}

		sendcnt += cnt
	}
}
