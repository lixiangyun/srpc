package main

import (
	"log"
	"os"

	"github.com/lixiangyun/srpc"
)

// 监听的端口和地址信息
const (
	DEFAULT_LISTEN_ADDR = ":1234" //
)

// 服务端对象引用
var server *srpc.Server

// 调用的对象
type SAVE struct {
	tmp uint32
}

// 对象的方法
func (s *SAVE) Add(a uint32, b *uint32) error {
	*b = a + s.tmp

	log.Println("call add ", a, *b, s.tmp)

	return nil
}

// 对象的方法
func (s *SAVE) Sub(a uint32, b *uint32) error {
	*b = a - s.tmp

	log.Println("call sub ", a, *b, s.tmp)

	return nil
}

// RPC服务端、申请、启动处理函数。
func Server(addr string) {

	log.Println("listen : ", addr)

	var s SAVE
	s.tmp = 100

	rpc_server := srpc.NewServer(addr)
	if rpc_server == nil {
		log.Println("new rpc server failed!")
		return
	}

	// 添加save对象的方法
	rpc_server.RegMethod(&s)

	// 启动rpc服务
	rpc_server.Start()
}

func main() {

	args := os.Args

	if len(args) > 1 {
		Server(args[2])
	} else {
		Server(DEFAULT_LISTEN_ADDR)
	}
}
