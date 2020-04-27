package main

import (
	. "cloud-client-go/client"
	. "cloud-client-go/util"
	"strings"
	"sync"
)

var (
	wg sync.WaitGroup
)

func main() {
	config := ReadConfig("asr.json")
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
		defer wg.Done()
		Send(client, config)

	}()

	//receive
	go func() {
		defer wg.Done()
		for true {
			buf := make([]byte, 10000)
			n, err := client.TcpConn.Read(buf)
			if err != nil {
				ConsoleLogger.Printf(err.Error())
			}
			ConsoleLogger.Printf("Get %d bytes", n)

			ss := strings.TrimSpace(string(buf[0:n]))
			ConsoleLogger.Printf(ss)
			if strings.HasSuffix(ss, "\r\n0") {
				break
			}
		}
	}()

	wg.Wait()
	ConsoleLogger.Println("Request Complete")
}
