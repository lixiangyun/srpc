# srpc
Simple &amp; High performance RPC for Golang Library

![](https://travis-ci.com/lixiangyun/srpc.svg?branch=master)

# 性能测试
环境 4U4G Intel(R) Xeon(R) CPU E5-2680 v4 @ 2.40GHz

# 测试结果
|并发数|kTPS|
|---|---|
|1|103.123|
|4|84.817|
|10|89.23|

## server 端代码示例
```
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

```

## client 端代码示例
```
package main

import (
	"log"

	"github.com/lixiangyun/srpc"
)

type InputParam struct {
	A32 uint32
	B32 []uint32
}

type OnputParam struct {
	Sum string
}

// 同步调用的rpc示例
func ClientSync(client *srpc.Client) {

	var in InputParam
	var out OnputParam

	// 参数初始化
	in.A32 = 1000
	in.B32 = make([]uint32, 100)
	for i := uint32(0); i < 100; i++ {
		in.B32[i] = i
	}

	// 同步调用
	err := client.Call("Add", in, &out)
	if err != nil {
		log.Println(err.Error())
		return
	}

	// 打印返回参数
	log.Println("Sync Call : out = ", out)
}

// 异步调用的rpc示例
func ClientAsync(client *srpc.Client) {

	var in InputParam
	var out OnputParam

	// 参数初始化
	in.A32 = 500
	in.B32 = make([]uint32, 50)
	for i := uint32(0); i < 50; i++ {
		in.B32[i] = i
	}

	// 创建异步的接收管道
	done := make(chan *srpc.Result, 1)

	// 异步调用
	client.CallAsync("Add", in, &out, done)

	// 等待结果
	result := <-done

	// 处理结果
	if result.Err != nil {
		log.Println("call method failed!", result)
		return
	}

	// 打印返回参数
	log.Println("Async Call : out = ", out)
}

func main() {

	// 创建rpc客户端对象
	client := srpc.NewClient("127.0.0.1:1200")
	if client == nil {
		log.Println("new client failed!")
		return
	}

	// 启动rpc客户端
	err := client.Start()
	if err != nil {
		log.Println(err.Error())
		return
	}

	defer client.Stop()

	ClientSync(client)
	ClientAsync(client)
}

```
