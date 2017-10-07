package main

import (
	"log"
	"srpc"
	"sync"
)

const (
	SERVER_ADDR = "localhost:1234"
)

func ClientSync(addr string) {

	client := srpc.NewClient(addr)
	if client == nil {
		log.Println("new client failed!")
		return
	}

	err := client.Start()
	if err != nil {
		log.Println(err.Error())
		return
	}

	var a, b uint32

	a = 100
	err = client.Call("Add", a, &b)
	if err != nil {
		log.Println(err.Error())
	} else {
		log.Println("a=", a, " b=", b)
	}

	a = 100
	err = client.Call("Sub", a, &b)
	if err != nil {
		log.Println(err.Error())
	} else {
		log.Println("a=", a, " b=", b)
	}

	client.Stop()
}

func Replay(rspque chan *srpc.Result, stop *sync.WaitGroup) {

	defer stop.Done()

	for i := 0; i < 10; i++ {
		result, b := <-rspque

		if b == false {
			log.Println("client replay chan close!")
			return
		}

		if result.Err != nil {
			log.Println("call method failed!", result)
		}

		log.Print("No.", i, " call ", result.Method, " return sucess! ")
		log.Println(" a =", result.Req, " b =", *(result.Rsp.(*uint32)))
	}
}

func ClientAsync(addr string) {

	var stop sync.WaitGroup

	client := srpc.NewClient(addr)
	if client == nil {
		log.Println("new client failed!")
		return
	}

	err := client.Start()
	if err != nil {
		log.Println(err.Error())
		return
	}

	var a, b uint32
	a = 1

	done := make(chan *srpc.Result, 10)

	stop.Add(1)
	go Replay(done, &stop)

	for i := 0; i < 10; i++ {
		a = uint32(i)
		client.CallAsync("Add", a, &b, done)
	}

	stop.Wait()

	client.Stop()
}

func main() {
	ClientSync(SERVER_ADDR)
	ClientAsync(SERVER_ADDR)
}
