package main

import (
	"bufio"
	. "cloud-client-go/handler"
	. "cloud-client-go/util"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	wg        sync.WaitGroup
	jsonType  = "json"
	audioType = "audio"
)

const (
	audioPartSize  = 640
	audioPartSleep = 20 * time.Millisecond
)

func main() {
	config := ReadConfig("asr.json")
	client := NewHttpV2Client(config.Host, config.Port, WithProtocol(config.Protocol), WithPath(config.Path))
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
		boundary := config.GetBoundary()
		if err := client.SendHeaders(config.Headers); err != nil {
			ConsoleLogger.Fatalln(err)
		}
		for _, part := range config.MultiParts {
			if part.Type == jsonType {
				bodyData, _ := json.Marshal(part.Body)
				if err := client.SendMultiPart(boundary, part.Parameters, bodyData); err != nil {
					ConsoleLogger.Fatalln(err)
				}
			}
			if part.Type == audioType {
				audioFile, _ := os.Open(fmt.Sprintf("%s", part.Body))

				r := bufio.NewReader(audioFile)
				b := make([]byte, audioPartSize)

				for true {
					n, er := r.Read(b)
					if err := client.SendMultiPart(boundary, part.Parameters, b[0:n]); err != nil {
						ConsoleLogger.Fatalln(err)
					}

					if er != nil {
						ConsoleLogger.Printf(er.Error())
						break
					}
					time.Sleep(audioPartSleep)
				}
			}
		}
		if err := client.SendMultiPartEnd(boundary); err != nil {
			ConsoleLogger.Fatalln(err)
		}

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

func getCurrentTime() string {
	t := time.Now()
	return t.Format("2006-01-02 15:04:05.000") + "\t"
}
