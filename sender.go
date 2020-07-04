package main

import (
	"bufio"
	. "cloud-client-go/config"
	"cloud-client-go/http_v2_client"
	. "cloud-client-go/util"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"
)

const Sending = "Sending:"

func Send(client *http_v2_client.HttpV2Client, config *Config) {
	if err := client.SendHeaders(config.Headers); err != nil {
		ConsoleLogger.Fatalln(err)
	}
	for _, part := range config.MultiParts {
		if part.Type == JsonType {
			sendJsonMsg(client, part)
		}
		if part.Type == AudioType {
			sendAudioMsg(client, part)
		}
	}
	ConsoleLogger.Println("Send Part End")
	if err := client.SendMultiPartEnd(); err != nil {
		ConsoleLogger.Fatalln(err)
	}
}

func sendJsonMsg(client *http_v2_client.HttpV2Client, part MultiPart) error {
	bodyData, _ := json.Marshal(part.Body)
	PrintPrettyJson(Sending, bodyData)
	if err := client.SendMultiPart(part.Parameters, bodyData); err != nil {
		ConsoleLogger.Fatalln(err)
		return err
	}
	return nil
}

func sendAudioMsg(client *http_v2_client.HttpV2Client, part MultiPart) error {
	audioFile, _ := os.Open(fmt.Sprintf("%s", part.Body))
	if part.StreamingEnable {
		sleep, err := time.ParseDuration(part.StreamTiming)
		if err != nil {
			ConsoleLogger.Fatal(fmt.Sprintf("invalid stream_timing:%s", part.StreamTiming))
		}
		reader := bufio.NewReader(audioFile)
		buffer := make([]byte, part.StreamSize)
		for {
			n, er := reader.Read(buffer)
			if n == 0 {
				break
			}
			if er != nil {
				if er == io.EOF {
					ConsoleLogger.Printf(er.Error())
					break
				} else {
					ConsoleLogger.Printf(er.Error())
					return er
				}
			}

			//ConsoleLogger.Println(fmt.Sprintf("%s %d bytes audio", Sending, n))
			if err := client.SendMultiPart(part.Parameters, buffer[0:n]); err != nil {
				ConsoleLogger.Fatalln(err)
			}
			time.Sleep(sleep)
		}

	} else {
		all, err := ioutil.ReadAll(audioFile)
		if err != nil {
			ConsoleLogger.Fatal(err.Error())
		}
		ConsoleLogger.Println(fmt.Sprintf("%s %d bytes audio", Sending, len(all)))
		if err := client.SendMultiPart(part.Parameters, all); err != nil {
			ConsoleLogger.Fatal(err.Error())
		}
	}
	return nil
}
