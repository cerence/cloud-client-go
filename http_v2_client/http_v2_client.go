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
	"strings"
	"sync"
	"time"
)

const CRLF = "\r\n"

const (
	HEADERS = iota
	CHUNK
	END
	NONE
)

const (
	Handing_Header = iota
	Handling_Chunk_Len
	Handling_Muti_Part_Header
	Handling_Muti_Part_Body
	Handing_Chunk_Body
	Handling_ChunkEnd
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
	revBuf    bytes.Buffer
	mutex     sync.Mutex
	revStatus int
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
			revBuf:    bytes.Buffer{},
			mutex:     sync.Mutex{},
			revStatus: Handing_Header,
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
	go c.ListenPort(ctx)

	headers, _ := c.handleHttpHeaders()
	ConsoleLogger.Println(headers)

	c.handleChunk(ctx)
	cancel()
}

func (c *HttpV2Client) handleHttpHeaders() ([]string, error) {
	//c.revResult.mutex.Lock()
	//defer c.revResult.mutex.Unlock()

	var headers []string
	for {
		line, err := c.revResult.revBuf.ReadBytes(0x0D)
		if err != nil {
			if err == io.EOF {
				continue
			}
			ConsoleLogger.Println(err.Error())
			return nil, err
		}
		if !IsBlankLine(line) {
			headers = append(headers, string(removeCRLF(line)))
		} else {
			c.revResult.revStatus = Handling_Chunk_Len
			return headers, nil
		}
	}

}

func (c *HttpV2Client) handleChunk(ctx context.Context) error {
	c.revResult.mutex.Lock()
	defer c.revResult.mutex.Unlock()
	for {
		line, err := c.revResult.revBuf.ReadBytes(0x0D)

		if err != nil {
			ConsoleLogger.Println(err.Error())
			return err
		}
		lineWithCRLF := removeCRLF(line)
		if lineWithCRLF != nil && string(lineWithCRLF) == "0" {
			ConsoleLogger.Println("no more data")
			break
		}
		if c.revResult.revStatus == Handling_Chunk_Len {
			if IsBlankLine(line) {
				c.revResult.revStatus = Handing_Chunk_Body
			} else {
				ConsoleLogger.Println("Receive Chunk ", string(line))
			}
		} else if c.revResult.revStatus == Handing_Chunk_Body {

		} else if c.revResult.revStatus == Handling_ChunkEnd {
			return nil
		}

	}
	return nil
}

func (c *HttpV2Client) ListenPort(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			break
		default:
			if err := c.readFromTcpConn(); err != nil {
				if err != io.EOF {
					ConsoleLogger.Fatalln(err.Error())
					//return err
				} else {
					time.Sleep(20 * time.Millisecond)
				}
			}
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
	c.revResult.mutex.Lock()
	defer c.revResult.mutex.Unlock()
	_, err = c.revResult.revBuf.Write(temp[0:n])
	if err != nil {
		return err
	}
	return err
}

func removeCRLF(line []byte) []byte {
	if line == nil {
		return nil
	}
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
