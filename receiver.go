package main

import (
	"bytes"
	"cloud-client-go/http_v2_client"
	. "cloud-client-go/http_v2_client"
	. "cloud-client-go/util"
	"fmt"
	"os"
	"strings"
)

const Receiving = "Receiving:"

func Receive(client *http_v2_client.HttpV2Client, output string, audio string) {
	go client.Receive()

	var outputFile *os.File
	var err error
	writeOutput := false
	if output != "" {
		outputFile, err = os.Create(output)
		if err != nil {
			ConsoleLogger.Println(err)
		} else {
			writeOutput = true
			defer outputFile.Close()
		}
	}

	var audioFile *os.File
	var err1 error
	writeAudio := false
	if audio != "" {
		audioFile, err1 = os.Create(output)
		if err1 != nil {
			ConsoleLogger.Println(err1)
		} else {
			writeAudio = true
			defer audioFile.Close()
		}
	}

	for header := range client.GetReceivedHttpHeaderChannel() {
		ConsoleLogger.Println(fmt.Sprintf("%s Header: %s", Receiving, header))
		if writeOutput {
			outputFile.WriteString(header + CRLF)
		}
	}
	if writeOutput {
		outputFile.WriteString(CRLF)
	}

	for chunk := range client.GetReceivedChunkChannel() {
		parameters, isAudio := handleBoundaryAndParameters(chunk.BoundaryAndParameters)
		if len(parameters) > 0 {
			ConsoleLogger.Println(fmt.Sprintf("%s multiple parts", Receiving))
			for n := range parameters {
				ConsoleLogger.Println(parameters[n])
				if writeOutput {
					outputFile.WriteString(parameters[n] + CRLF)
				}
			}
		}

		if writeOutput {
			outputFile.WriteString(CRLF)
		}

		if isAudio {
			ConsoleLogger.Println(fmt.Sprintf("%s %d audio bytes", Receiving, chunk.Body.Len()))
			if writeAudio {
				audioFile.Write(chunk.Body.Bytes())
			}
		} else {
			PrintPrettyJson(Receiving, chunk.Body.Bytes())
		}
		if writeOutput {
			json := PrintPrettyJson(Receiving, chunk.Body.Bytes())
			outputFile.WriteString(json + CRLF)
		}
	}
}

func handleBoundaryAndParameters(bytes bytes.Buffer) ([]string, bool) {
	data := strings.Split(bytes.String(), CRLF)
	var parameters []string
	isAudioPart := true
	for n := range data {
		if strings.Trim(data[n], "\r") != "" {
			parameters = append(parameters, data[n])
			if strings.Contains(data[n], "Content-Type: application/JSON;") {
				isAudioPart = false
			}
		}
	}
	return parameters, isAudioPart
}
