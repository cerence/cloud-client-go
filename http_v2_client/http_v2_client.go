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
	Handing_Header = iota
	Handling_Chunk_Len
	Handing_Boundary_Parameters
	Handing_Chunk_Body
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
	receiveEnable chan int
	handleEnable  chan int

	headers       bytes.Buffer
	handlingChunk *Chunk
	receivedChunk chan Chunk

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
			revBuf:        bytes.Buffer{},
			receiveEnable: make(chan int, 1),
			handleEnable:  make(chan int, 1),
			revStatus:     Handing_Header,
			headers:       bytes.Buffer{},
			receivedChunk: make(chan Chunk, 10),
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
	c.revResult.revStatus = Handing_Header
	c.revResult.receiveEnable <- 1

	go c.listenPort(ctx)
	c.handleResponse(ctx)
	cancel()
	close(c.revResult.receivedChunk)
	close(c.revResult.handleEnable)
	close(c.revResult.receiveEnable)

}

func (c *HttpV2Client) GetReceivedChunkChannel() chan Chunk {
	return c.revResult.receivedChunk
}

func (c *HttpV2Client) handleResponse(ctx context.Context) {
	select {
	case <-ctx.Done():
		return

	default:
		for {
			<-c.revResult.handleEnable

			c.revResult.resetCRLF()
			data := c.revResult.revBuf.Next(c.revResult.revBuf.Len())

			for i := 0; i < len(data); i++ {
				b := data[i]
				c.revResult.handleCRLF(b)

				switch c.revResult.revStatus {
				case Handing_Header:
					c.revResult.handleHeader(b)
				case Handling_Chunk_Len:
					if !c.revResult.handleChunkLen(b) {
						return
					}
				case Handing_Boundary_Parameters:
					c.revResult.handleBoundaryParameters(b)
				case Handing_Chunk_Body:
					c.revResult.handleChunkBody(b)
				}
			}
			c.revResult.receiveEnable <- 1
		}
	}
}

func (c *HttpV2Client) listenPort(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			ConsoleLogger.Println("Exiting listening")
			break
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
		c.revResult.handleEnable <- 1
	}
	return err
}

func (r *result) shouldHandleNext() bool {
	if r.handlingChunk.ReceivedLen >= r.handlingChunk.Len {
		r.handlingChunk = nil
		r.revStatus = Handling_Chunk_Len
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

func (r *result) handleHeader(b byte) {
	if r.doubleCRLF {
		r.revStatus = Handling_Chunk_Len

	} else {
		r.headers.Write([]byte{b})

	}
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
		r.revStatus = Handing_Boundary_Parameters
		if r.handlingChunk.Len == 0 {
			r.receivedChunk <- *r.handlingChunk
			r.handlingChunk = nil
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
		r.revStatus = Handing_Chunk_Body
	}
	r.shouldHandleNext()
}

func (r *result) handleChunkBody(b byte) {
	r.handlingChunk.ReceivedLen++
	r.handlingChunk.Body.Write([]byte{b})
	r.shouldHandleNext()

}
