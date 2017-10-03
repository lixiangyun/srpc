package main

import (
	"log"
	"srpc"
)

const (
	LISTEN_ADDR = ":1234"
)

var server *srpc.Server

type SAVE struct {
	tmp uint32
}

func (s *SAVE) Add(a uint32, b *uint32) error {
	*b = a + s.tmp

	log.Println("call add ", a, *b, s.tmp)

	return nil
}

func (s *SAVE) Sub(a uint32, b *uint32) error {
	*b = a - s.tmp

	log.Println("call sub ", a, *b, s.tmp)

	return nil
}

func Server(addr string) {

	var s SAVE
	s.tmp = 100

	rpc_server := srpc.NewServer(addr)
	if rpc_server == nil {
		log.Println("new rpc server failed!")
		return
	}

	rpc_server.RegMethod(&s)
	rpc_server.Start()
}

func main() {
	Server(LISTEN_ADDR)
}
