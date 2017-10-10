package main

import (
	"log"
	"os"
	"runtime"
	"srpc"
	"strconv"
	"sync"
)

var wait sync.WaitGroup

const (
	MAX_MSGID = 1000000
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
		log.Println(err.Error())
	} else {
		log.Println("a=", a, " b=", b)
	}

	log.Println("start...")

	for i := 0; i < 10000; i++ {
		a = uint32(i)
		err = client.Call("Add", a, &b)
		if err != nil {
			log.Println(err.Error())
			return
		}
	}

	log.Println("end...")
}

func Replay(rspque chan *srpc.Result, stop *sync.WaitGroup) {

	defer stop.Done()

	var totalcnt int

	for {
		result, b := <-rspque

		if b == false {
			log.Println("client replay chan close!")
			return
		}

		if result.Err != nil {
			log.Println("call method failed!", result)
		}

		totalcnt++

		if totalcnt >= MAX_MSGID {
			log.Println("recv total num : ", totalcnt)
			return
		}
	}
}

func ClientAsync(addr string) {

	var stop sync.WaitGroup

	defer wait.Done()

	client := srpc.NewClient(addr)
	if client == nil {
		return
	}

	err := client.Start()
	if err != nil {
		log.Println(err.Error())
		return
	}

	var a, b uint32
	a = 1

	done := make(chan *srpc.Result, 1000)

	stop.Add(1)
	go Replay(done, &stop)

	for i := 0; i < MAX_MSGID; i++ {
		a = uint32(i)
		client.CallAsync("Add", a, &b, done)
	}

	stop.Wait()

	client.Stop()
}

func main() {

	addr := "localhost:1234"

	cpunum := runtime.NumCPU()
	runtime.GOMAXPROCS(cpunum)

	log.Println("max cpu num: ", cpunum)

	num := cpunum

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
