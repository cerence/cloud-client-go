package main

import (
	"cloud-client-go/util"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Protocol   string      `json:"protocol"`
	Host       string      `json:"host"`
	Port       int         `json:"port"`
	Path       string      `json:"path"`
	Headers    []string    `json:"headers"`
	MultiParts []MultiPart `json:"multi-part"`
	boundary   string
}

type MultiPart struct {
	Type       string      `json:"type"`
	Parameters []string    `json:"parameters"`
	Body       interface{} `json:"body"`
}

func (c *Config) GetBoundary() string {
	if strings.Compare(c.boundary, "") != 0 {
		return c.boundary
	}
	for _, header := range c.Headers {
		if strings.HasPrefix(header, "Content-Type: multipart/form-data; boundary=") {
			c.boundary = strings.Replace(header, "Content-Type: multipart/form-data; boundary=", "", 1)
			return c.boundary
		}
	}
	c.Headers = append(c.Headers, "Content-Type: multipart/form-data; boundary=sk29ksksk82ksmsgdfg4rgs5llopsja82")
	c.boundary = "sk29ksksk82ksmsgdfg4rgs5llopsja82"
	return c.boundary
}

func ReadConfig(path string) *Config {
	file, err := os.Open(path)
	if err != nil {
		util.ConsoleLogger.Fatalln(fmt.Sprintf("Can't open config file: %s", err.Error()))
	}
	defer file.Close()

	var request *Config
	if err = json.NewDecoder(file).Decode(&request); err != nil {
		util.ConsoleLogger.Fatalln("Fail to decode the input file", err)
	}

	return request
}
