package main

import (
	config2 "cloud-client-go/config"
	. "cloud-client-go/http_v2_client"
	. "cloud-client-go/util"
	"sync"
)

var (
	wg sync.WaitGroup
)

func main() {
	config := config2.ReadConfig("asr.json")
	client := NewHttpV2Client(config.Host, config.Port, WithProtocol(config.Protocol), WithPath(config.Path), WithBoundary(config.GetBoundary()))
	if client == nil {
		ConsoleLogger.Fatalln("Can't new connection")
	}

	if err := client.Connect(); err != nil {
		ConsoleLogger.Fatalln("Can't connect to server")
	}
	defer client.Close()

	wg.Add(2)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				ConsoleLogger.Println(err)
			}
		}()
		defer wg.Done()
		Send(client, config)
		ConsoleLogger.Println("Send done")
	}()

	//receive
	go func() {
		defer wg.Done()
		client.Receive()
		//for true {
		//	buf := make([]byte, 10000)
		//	n, err := client.TcpConn.Read(buf)
		//	if err != nil {
		//		ConsoleLogger.Printf(err.Error())
		//	}
		//
		//
		//	ss := strings.TrimSpace(string(buf[0:n]))
		//	ConsoleLogger.Printf(ss)
		//	if strings.HasSuffix(ss, "\r\n0") {
		//		break
		//	}
		//}
		ConsoleLogger.Println("Receive done")
	}()

	wg.Wait()
	ConsoleLogger.Println("Request Complete")
}
