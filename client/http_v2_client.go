package client

import (
	"bytes"
	. "cloud-client-go/util"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const CRLF = "\r\n"

const (
	HEADERS = iota
	CHUNK
	END
	NONE
)

type HttpV2Client struct {
	Protocol string
	Host     string
	Port     int
	Path     string
	Timeout  time.Duration
	TcpConn  net.Conn
	boundary string
	revBuf   bytes.Buffer
}

type optionFunc struct {
	f func(client *HttpV2Client)
}

func NewHttpV2Client(Host string, Port int, opts ...HttpV2Option) *HttpV2Client {
	var client = &HttpV2Client{
		Protocol: "https",
		Host:     Host,
		Port:     Port,
		Path:     "/NmspServlet/",
		Timeout:  10 * time.Second,
		TcpConn:  nil,
		revBuf:   bytes.Buffer{},
		boundary: DefaultBoundary,
	}
	for _, opt := range opts {
		opt.apply(client)
	}
	return client
}

func (c *HttpV2Client) Connect() error {
	ConsoleLogger.Println(fmt.Sprintf("Connecting %s://%s:%d%s", c.Protocol, c.Host, c.Port, c.Path))
	if strings.ToLower(c.Protocol) == "https" {
		c.TcpConn, _ = tls.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port), &tls.Config{InsecureSkipVerify: true})
	} else if strings.ToLower(c.Protocol) == "http" {
		c.TcpConn, _ = net.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port))
	} else {
		return errors.New("unknown protocol")
	}
	c.TcpConn.SetDeadline(time.Now().Add(c.Timeout))
	ConsoleLogger.Println(fmt.Sprintf("Connected %s://%s:%d%s", c.Protocol, c.Host, c.Port, c.Path))
	return nil

}

func (c *HttpV2Client) SendHeaders(headers []string) error {
	if err := c.checkConnection(); err != nil {
		return err
	}

	if _, err := c.TcpConn.Write([]byte(fmt.Sprintf("%s /%s/ HTTP/1.1%s", "POST", c.Path, CRLF))); err != nil {
		return err
	}
	for _, v := range headers {
		if _, err := c.TcpConn.Write([]byte(fmt.Sprintf("%s%s", v, CRLF))); err != nil {
			return err
		}
	}
	_, err := c.TcpConn.Write([]byte(CRLF))
	return err
}

func (c *HttpV2Client) SendMultiPart(parameters []string, body []byte) error {
	if err := c.checkConnection(); err != nil {
		return err
	}

	var buf bytes.Buffer
	defer buf.Reset()
	if _, err := buf.Write([]byte(CRLF + fmt.Sprintf("--%s%s", c.boundary, CRLF))); err != nil {
		return err
	}
	for _, v := range parameters {
		if _, err := buf.Write([]byte(v + CRLF)); err != nil {
			return err
		}
	}
	if _, err := buf.Write([]byte(CRLF)); err != nil {
		return err
	}
	if _, err := buf.Write(body); err != nil {
		return err
	}

	return c.sendChunk(buf.Bytes())
}

func (c *HttpV2Client) SendMultiPartEnd() error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	if err := c.sendChunk([]byte(fmt.Sprintf("%s--%s--%s", CRLF, c.boundary, CRLF))); err != nil {
		return err
	}
	return c.sendChunkEnd()
}

func (c *HttpV2Client) sendChunk(body []byte) error {
	//Write length
	if _, err := c.TcpConn.Write([]byte(fmt.Sprintf("%x", len(body)))); err != nil {
		return err
	}
	if _, err := c.TcpConn.Write([]byte(CRLF)); err != nil {
		return err
	}
	//Write body
	if _, err := c.TcpConn.Write(body); err != nil {
		return err
	}
	_, err := c.TcpConn.Write([]byte(CRLF))
	return err
}

func (c *HttpV2Client) sendChunkEnd() error {
	_, err := c.TcpConn.Write([]byte("0" + CRLF + CRLF))
	return err
}

func (c *HttpV2Client) checkConnection() error {
	if c.TcpConn == nil {
		return errors.New("call Connect method firstly")
	}
	return nil
}

func (c *HttpV2Client) Close() error {
	if c.TcpConn == nil {
		return nil
	}

	return c.TcpConn.Close()
}

func (c *HttpV2Client) ReceiveChunk() (msgType int, data []byte, err error) {
	isHeader := false
	isFirstLine := true
	var chunkSize int64 = 0
	var totalReceiveSize int64 = 0
	temp := bytes.Buffer{}
	defer temp.Reset()
	readFromConn(c)
	for true {
		line, err := c.revBuf.ReadBytes(0x0D)

		if err != nil {
			if err.Error() == "EOF" {
				if isHeader {
					return HEADERS, temp.Bytes(), nil
				} else {
					n := readFromConn(c)
					if n == 0 {
						break
					}
				}
			} else {
				return 0, nil, err
			}
		}

		if isFirstLine {
			isFirstLine = false
			if strings.HasPrefix(string(line), "HTTP/1.1") {
				isHeader = true
			} else {
				isHeader = false
				line = removeCRLF(line)
				chunkSize, _ = strconv.ParseInt(string(line), 16, 64)
				if len(line) == 1 && chunkSize == 0 {
					return END, nil, nil
				}
				continue
			}
		}

		if isHeader {
			if IsBlankLine(line) {
				return HEADERS, temp.Bytes(), nil
			} else {
				temp.Write(line)
			}
		} else {
			receiveSize := len(line)
			if receiveSize == 0 {
				n := readFromConn(c)
				if n == 0 {
					break
				}
			}
			totalReceiveSize += int64(receiveSize)
			temp.Write(line)
			if totalReceiveSize >= chunkSize+2 {
				return CHUNK, temp.Bytes(), nil
			}
		}
	}

	return NONE, nil, nil
}

func readFromConn(c *HttpV2Client) int {
	buf := make([]byte, 10000)
	n, _ := c.TcpConn.Read(buf)
	c.revBuf.Write(buf[0:n])
	return n
}

func removeCRLF(line []byte) []byte {
	temp := bytes.Buffer{}
	for _, v := range line {
		if v != 0x0A && v != 0x0D {
			temp.WriteByte(v)
		}
	}

	return temp.Bytes()
}

func IsBlankLine(line []byte) bool {
	if len(line) == 1 && (line[0] == 0x0A || line[0] == 0x0D) {
		return true
	}
	if len(line) == 2 && line[0] == 0x0A && line[1] == 0x0D {
		return true
	}
	if len(line) == 2 && line[0] == 0x0D && line[1] == 0x0A {
		return true
	}
	return false
}
