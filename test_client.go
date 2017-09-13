package main

import (
	"fmt"
	"golang_demo/srpc/srpc"
	"log"
	//"os"
	//"runtime/pprof"
	"sync"
)

var wait sync.WaitGroup

func Client(addr string) {

	defer wait.Done()

	client := srpc.NewClient(addr)

	var a, b uint32
	a = 1
	err := client.Call("Add", a, &b)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("a=", a, " b=", b)
	}

	log.Println("start...")

	for i := 0; i < 10000; i++ {
		a = uint32(i)
		err = client.Call("Add", a, &b)
		if err != nil {
			fmt.Println(err.Error())
		}
	}

	log.Println("end...")
}

func main() {

	//f, _ := os.Create("profile_file")
	//pprof.StartCPUProfile(f)     // 开始cpu profile，结果写到文件f中
	//defer pprof.StopCPUProfile() // 结束profile

	wait.Add(10)

	for i := 0; i < 10; i++ {
		go Client("localhost:1234")
	}

	wait.Wait()
}
