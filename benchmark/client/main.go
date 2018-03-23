package main

import (
	"log"
	"os"
	"strconv"
	"sync"

	"github.com/lixiangyun/srpc"
)

var wait sync.WaitGroup

func ClientSync(client *srpc.Client) {

	defer wait.Done()

	var a, b uint32
	a = 1

	i := 0
	for {
		a = uint32(i)

		err := client.Call("Add", a, &b)
		if err != nil {
			log.Println(err.Error())
			break
		}

		if a != b {
			log.Println("failed.")
			break
		}

		i++
	}
}

func main() {

	addr := "127.0.0.1:1234"

	num := 100

	args := os.Args

	if len(args) == 3 {
		num, _ = strconv.Atoi(args[1])
		addr = args[2]
	}

	log.Println("num : ", num)
	log.Println("addr : ", addr)

	client := srpc.NewClient(addr)
	if client == nil {
		return
	}

	err := client.Start()
	if err != nil {
		log.Println(err.Error())
		return
	}

	wait.Add(num)

	for i := 0; i < num; i++ {
		go ClientSync(client)
	}

	wait.Wait()

	client.Stop()
}
