package http_v2_client

import (
	"bytes"
	. "cloud-client-go/util"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const CRLF = "\r\n"
const CR = '\r'
const LF = '\n'

const (
	handingHeader = iota
	handlingChunkLen
	handingBoundaryParameters
	handingChunkBody
)

type HttpV2Client struct {
	Protocol  string
	Host      string
	Port      int
	Path      string
	Timeout   time.Duration
	TcpConn   net.Conn
	boundary  string
	revResult result
}

type result struct {
	revBuf        bytes.Buffer
	revStatus     int
	receiveEnable chan bool
	handleEnable  chan bool

	headers        bytes.Buffer
	handlingChunk  *Chunk
	receivedChunk  chan Chunk
	receivedHeader chan string

	getCR      bool
	getLF      bool
	doubleCRLF bool
}

type Chunk struct {
	LenByte               bytes.Buffer
	Len                   int64
	BoundaryAndParameters bytes.Buffer
	Body                  bytes.Buffer
	ReceivedLen           int64
}

func NewChunk() *Chunk {
	return &Chunk{
		LenByte:               bytes.Buffer{},
		BoundaryAndParameters: bytes.Buffer{},
		Body:                  bytes.Buffer{},
		ReceivedLen:           0,
	}
}

func NewHttpV2Client(Host string, Port int, opts ...Option) *HttpV2Client {
	var client = &HttpV2Client{
		Protocol: "https",
		Host:     Host,
		Port:     Port,
		Path:     "/NmspServlet/",
		Timeout:  60 * time.Second,
		TcpConn:  nil,
		boundary: DefaultBoundary,
		revResult: result{
			revBuf:         bytes.Buffer{},
			receiveEnable:  make(chan bool, 1),
			handleEnable:   make(chan bool, 1),
			revStatus:      handingHeader,
			headers:        bytes.Buffer{},
			receivedChunk:  make(chan Chunk, 30),
			receivedHeader: make(chan string, 20),
		},
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

func (c *HttpV2Client) Receive() {

	ctx, cancel := context.WithCancel(context.Background())
	c.revResult.revStatus = handingHeader
	c.revResult.receiveEnable <- true
	go c.listenPort(ctx)
	c.handleResponse()

	defer cancel()
	close(c.revResult.receivedChunk)
	//close(c.revResult.handleEnable)
	//close(c.revResult.receiveEnable)

}

func (c *HttpV2Client) GetReceivedChunkChannel() chan Chunk {
	return c.revResult.receivedChunk
}

func (c *HttpV2Client) GetReceivedHttpHeaderChannel() chan string {
	return c.revResult.receivedHeader
}

func (c *HttpV2Client) handleResponse() {

	for {
		if !<-c.revResult.handleEnable {
			close(c.revResult.receivedHeader)
			return
		}

		c.revResult.resetCRLF()
		data := c.revResult.revBuf.Next(c.revResult.revBuf.Len())

		for i := 0; i < len(data); i++ {
			b := data[i]
			c.revResult.handleCRLF(b)

			switch c.revResult.revStatus {
			case handingHeader:
				if !c.revResult.handleHeader(b) {
					return
				}
			case handlingChunkLen:
				if !c.revResult.handleChunkLen(b) {
					return
				}
			case handingBoundaryParameters:
				c.revResult.handleBoundaryParameters(b)
			case handingChunkBody:
				c.revResult.handleChunkBody(b)
			}
		}
		c.revResult.receiveEnable <- true
	}

}

func (c *HttpV2Client) listenPort(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			ConsoleLogger.Println("Exiting listening")
			c.revResult.handleEnable <- false
			return ctx.Err()
		case <-c.revResult.receiveEnable:
			if err := c.readFromTcpConn(); err != nil {
				if err != io.EOF {
					ConsoleLogger.Fatalln(err.Error())
				}
			}
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	return nil
}

func (c *HttpV2Client) readFromTcpConn() error {
	temp := make([]byte, 1024)
	n, err := c.TcpConn.Read(temp)
	if err != nil {
		return err
	}
	if n > 0 {
		_, err = c.revResult.revBuf.Write(temp[0:n])
		if err != nil {
			return err
		}
		c.revResult.handleEnable <- true
	}
	return err
}

func (r *result) shouldHandleNext() bool {
	if r.handlingChunk.ReceivedLen >= r.handlingChunk.Len {
		r.receivedChunk <- *r.handlingChunk
		r.handlingChunk = nil
		r.revStatus = handlingChunkLen
		return true
	} else {
		return false
	}
}

func (r *result) resetCRLF() {
	r.getCR = false
	r.getLF = false
	r.doubleCRLF = false
}

func (r *result) handleCRLF(b byte) {
	if b == CR {
		r.getCR = true
	} else if b == LF {
		if r.getCR && r.getLF {
			r.doubleCRLF = true
		} else {
			r.getLF = true
		}
	} else {
		r.getLF = false
		r.getCR = false
		r.doubleCRLF = false
	}
}

func (r *result) handleHeader(b byte) bool {
	if r.doubleCRLF {
		isHttp200 := false
		headers := strings.Split(r.headers.String(), CRLF)
		for n := range headers {
			if strings.Trim(headers[n], "\r") != "" {
				if strings.Contains(headers[n], "HTTP/1.1 200") {
					isHttp200 = true
				}
				r.receivedHeader <- headers[n]
			}
		}
		close(r.receivedHeader)
		r.revStatus = handlingChunkLen
		return isHttp200

	} else {
		r.headers.Write([]byte{b})

	}
	return true
}

func (r *result) handleChunkLen(b byte) bool {
	if r.handlingChunk == nil {
		r.handlingChunk = NewChunk()
	}
	if !r.getLF && !r.getCR {
		r.handlingChunk.LenByte.Write([]byte{b})
	} else {
		if r.handlingChunk.LenByte.Len() == 0 {
			return true
		}
		r.handlingChunk.Len, _ = strconv.ParseInt(r.handlingChunk.LenByte.String(), 16, 64)
		r.revStatus = handingBoundaryParameters
		if r.handlingChunk.Len == 0 {
			return false
		}
		return true
	}
	return true
}

func (r *result) handleBoundaryParameters(b byte) {
	if b == '-' || r.handlingChunk.BoundaryAndParameters.Len() != 0 {
		r.handlingChunk.ReceivedLen++
		r.handlingChunk.BoundaryAndParameters.Write([]byte{b})
	}
	if r.doubleCRLF && r.handlingChunk.BoundaryAndParameters.Len() != 0 {
		r.revStatus = handingChunkBody
	}
	r.shouldHandleNext()
}

func (r *result) handleChunkBody(b byte) {
	r.handlingChunk.ReceivedLen++
	r.handlingChunk.Body.Write([]byte{b})
	r.shouldHandleNext()

}
