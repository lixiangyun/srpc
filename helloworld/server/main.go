package main

import (
	"fmt"
	"log"

	"github.com/lixiangyun/srpc"
)

// 调用的对象
type ServerObj struct {
}

type InputParam struct {
	A32 uint32
	B32 []uint32
}

type OnputParam struct {
	Sum string
}

// 对象的方法
func (s *ServerObj) Add(in InputParam, out *OnputParam) error {

	var sum uint32

	for _, v := range in.B32 {
		sum += v
	}

	out.Sum = fmt.Sprintf("no : %d , sum : %d", in.A32, sum)

	log.Println("call inparm ", in, " outparm", out)

	return nil
}

func main() {
	// svc对象初始化
	svc := ServerObj{}

	// 创建rpc服务对象
	rpc_server := srpc.NewServer("127.0.0.1:1200")
	if rpc_server == nil {
		log.Println("new rpc server failed!")
		return
	}

	// 将svc对象添加到rpc方法中
	rpc_server.RegMethod(&svc)

	// 启动rpc服务，并且阻塞运行
	rpc_server.Start()
}
