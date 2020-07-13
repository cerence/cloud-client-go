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

	outputFile, writeOutput := createFile(output)
	if writeOutput {
		defer outputFile.Close()
	}
	audioFile, writeAudio := createFile(audio)
	if writeOutput {
		defer audioFile.Close()
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
			if writeOutput {
				outputFile.WriteString(fmt.Sprintf("%s %d audio bytes", Receiving, chunk.Body.Len()))
			}
			if writeAudio {
				audioFile.Write(chunk.Body.Bytes())
			}
		} else {
			PrintPrettyJson(Receiving, chunk.Body.Bytes())
			if writeOutput {
				json := PrintPrettyJson(Receiving, chunk.Body.Bytes())
				outputFile.WriteString(json + CRLF)
			}
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

func createFile(fileName string) (*os.File, bool) {
	var file *os.File
	var err error
	enable := false
	if fileName != "" {
		file, err = os.Create(fileName)
		if err != nil {
			ConsoleLogger.Println(err)
		} else {
			enable = true
		}
	}

	return file, enable
}
