package main

import (
	"bufio"
	. "cloud-client-go/client"
	. "cloud-client-go/util"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"
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
		if err := client.SendHeaders(config.Headers); err != nil {
			ConsoleLogger.Fatalln(err)
		}
		for _, part := range config.MultiParts {
			if part.Type == JsonType {
				bodyData, _ := json.Marshal(part.Body)
				if err := client.SendMultiPart(part.Parameters, bodyData); err != nil {
					ConsoleLogger.Fatalln(err)
				}
			}
			if part.Type == AudioType {
				audioFile, _ := os.Open(fmt.Sprintf("%s", part.Body))
				if part.StreamingEnable {
					sleep, err := time.ParseDuration(part.StreamTiming)
					if err != nil {
						ConsoleLogger.Fatal(fmt.Sprintf("invalid stream_timing:%s", part.StreamTiming))
					}
					r := bufio.NewReader(audioFile)
					b := make([]byte, part.StreamSize)
					for {
						n, er := r.Read(b)
						if err := client.SendMultiPart(part.Parameters, b[0:n]); err != nil {
							ConsoleLogger.Fatalln(err)
						}

						if er != nil {
							ConsoleLogger.Printf(er.Error())
							break
						}
						time.Sleep(sleep)
					}
				} else {
					all, err := ioutil.ReadAll(audioFile)
					if err != nil {
						ConsoleLogger.Fatal(err.Error())
					}
					client.SendMultiPart(part.Parameters, all)
				}
			}
		}
		if err := client.SendMultiPartEnd(); err != nil {
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
