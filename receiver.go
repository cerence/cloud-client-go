package main

import (
	"cloud-client-go/http_v2_client"
	"fmt"
)

func Receive(client *http_v2_client.HttpV2Client, output string, audio string) {
	go client.Receive()
	for chunk := range client.GetReceivedChunkChannel() {
		fmt.Println(chunk.Body)
	}
}
