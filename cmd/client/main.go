package main

import (
	"log"

	"github.com/Terry-Mao/goim/goimClient"
	"github.com/bilibili/discovery/naming"
)

func main() {
	mid := int64(1)
	cometServer := "tcp://127.0.0.1:3101"
	disConf := &goimClient.Discovery{
		naming.Config{
			Nodes:  []string{"127.0.0.1:7171"},
			Region: "",
			Zone:   "sh001",
		},
	}
	options := append([]goimClient.Option(nil), goimClient.WithAccepts([]int32{1000, 2000, 3000}), goimClient.WithPlatform("golang client"))
	options = append(options, disConf.Options()...)
	client, err := goimClient.NewClient(cometServer, mid, "key1", "room1", options...)
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

	//curl -d 'mid message' http://127.0.0.1:3111/goim/push/mids?operation=1000&mids=1
	//send msg test
	err = client.SendMidMsgToServer("127.0.0.1:3111", []int64{mid}, 1000, "mid message test")
	if err != nil {
		log.Printf("SendMidMsgToServer err:%+v", err)
	}

	// use discovry find goim.logic http server
	err = client.SendMidMsg([]int64{mid}, 1000, "mid message test")
	if err != nil {
		log.Printf("SendMidMsg err:%+v", err)
	}

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
