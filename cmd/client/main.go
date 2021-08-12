package main

import (
	"log"

	"github.com/Terry-Mao/goim/goimClient"
)

func main() {
	mid := int64(1)
	client, err := goimClient.NewClient("tcp://127.0.0.1:3101", mid, "key1", "room1",
		goimClient.WithAccepts([]int32{1000, 2000, 3000}), goimClient.WithPlatform("golang client"))
	if err != nil {
		log.Panicf("%+v", err) //printf stack
	}
	err = client.Start()
	if err != nil {
		log.Panicf("%+v", err) //printf stack
	}

	defer func() {
		err := client.Close()
		log.Printf("client.Close() err:%+v\n", err)
	}()

	for {
		b := make([]byte, 1024)
		n, err := client.Read(b)
		if err != nil {
			log.Printf("Read fail: %+v\n", err) //printf stack
			return
		}
		log.Printf("recvmsg:%s\n", b[:n])
	}
}
