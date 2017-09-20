package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"srpc"
	"strconv"
	"sync"
	"time"
)

var wait sync.WaitGroup

const (
	MAX_MSGID = 100000
)

func Client(addr string) {

	defer wait.Done()

	client := srpc.NewClient(addr)
	if client == nil {
		return
	}

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
			return
		}
	}

	log.Println("end...")
}

func Replay(c *srpc.Client) {
	que := c.GetRelay()

	for {
		result, b := <-que

		if b == false {
			fmt.Println("client replay chan close!")
			return
		}

		if result.MsgId >= MAX_MSGID {
			fmt.Println("client test end!")
			return
		}
	}
}

func ClientAsync(addr string) {

	defer wait.Done()

	client := srpc.NewClient(addr)
	if client == nil {
		return
	}

	var a, b uint32
	a = 1

	log.Println("start...")

	go Replay(client)

	for i := 0; i < MAX_MSGID/100; i++ {
		for j := 0; j < 100; j++ {
			a = uint32(i)
			err := client.CallAsync("Add", a, &b)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		}
		time.Sleep(time.Millisecond)
	}

	client.Delete()

	log.Println("end...")
}

func main() {

	addr := "localhost:1234"

	cpunum := runtime.NumCPU()
	runtime.GOMAXPROCS(cpunum)

	log.Println("max cpu num: ", cpunum)

	num := 1

	args := os.Args

	if len(args) == 3 {
		num, _ = strconv.Atoi(args[1])
		addr = args[2]
	}

	log.Println("num : ", num)
	log.Println("addr : ", addr)

	wait.Add(num)

	for i := 0; i < num; i++ {
		go ClientAsync(addr)
	}

	wait.Wait()
}
