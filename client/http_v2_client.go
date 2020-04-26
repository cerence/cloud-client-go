package client

import (
	"bytes"
	. "cloud-client-go/util"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

const CRLF = "\r\n"

type HttpV2Option interface {
	apply(client *HttpV2Client)
}

type HttpV2Client struct {
	Protocol string
	Host     string
	Port     int
	Path     string
	Timeout  time.Duration
	TcpConn  net.Conn
	revBuf   bytes.Buffer
	boundary string
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

func WithProtocol(s string) HttpV2Option {
	return &optionFunc{
		f: func(client *HttpV2Client) {
			client.Protocol = s
		},
	}
}

func WithPath(s string) HttpV2Option {
	return &optionFunc{
		f: func(client *HttpV2Client) {
			client.Path = s
		},
	}
}

func WithBoundary(s string) HttpV2Option {
	return &optionFunc{
		f: func(client *HttpV2Client) {
			client.boundary = s
		},
	}
}

func (fdo *optionFunc) apply(do *HttpV2Client) {
	fdo.f(do)
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
