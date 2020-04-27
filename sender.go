package main

import (
	"bufio"
	"cloud-client-go/client"
	. "cloud-client-go/util"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"
)

const Sending = "Sending:"

func Send(client *client.HttpV2Client, config *Config) {
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
	if err := client.SendMultiPartEnd(); err != nil {
		ConsoleLogger.Fatalln(err)
	}
}

func sendJsonMsg(client *client.HttpV2Client, part MultiPart) error {
	bodyData, _ := json.Marshal(part.Body)
	PrintPrettyJson(Sending, bodyData)
	if err := client.SendMultiPart(part.Parameters, bodyData); err != nil {
		ConsoleLogger.Fatalln(err)
		return err
	}
	return nil
}

func sendAudioMsg(client *client.HttpV2Client, part MultiPart) error {
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
			if er != nil {
				ConsoleLogger.Printf(er.Error())
				break
			}
			ConsoleLogger.Println(fmt.Sprintf("%s %d bytes audio", Sending, n))
			if err := client.SendMultiPart(part.Parameters, b[0:n]); err != nil {
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
