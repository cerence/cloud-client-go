package main

import (
	. "cloud-client-go/util"
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
	Type            string      `json:"type"`
	Parameters      []string    `json:"parameters"`
	Body            interface{} `json:"body"`
	StreamingEnable bool        `json:"stream_enable"`
	StreamSize      int         `json:"stream_size"`
	StreamTiming    string      `json:"stream_timing"`
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
	c.Headers = append(c.Headers, fmt.Sprintf("Content-Type: multipart/form-data; boundary=%s", DefaultBoundary))
	c.boundary = DefaultBoundary
	return c.boundary
}

func ReadConfig(path string) *Config {
	file, err := os.Open(path)
	if err != nil {
		ConsoleLogger.Fatalln(fmt.Sprintf("Can't open config file: %s", err.Error()))
	}
	defer file.Close()

	var request *Config
	if err = json.NewDecoder(file).Decode(&request); err != nil {
		ConsoleLogger.Fatalln("Fail to decode the input file", err)
	}

	return request
}
