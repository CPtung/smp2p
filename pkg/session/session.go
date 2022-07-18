package session

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

type Session interface {
	Create() (err error)
	OnOpen(w io.Writer)
	OnMessage(data []byte)
	OnClose()
}

type Server struct {
	hostName string
	conn     net.Conn
}

func (s *Server) Create() (err error) {
	if s.conn, err = net.DialTimeout("tcp", s.hostName, time.Duration(3)*time.Second); err != nil {
		return err
	}
	return nil
}

func (s *Server) OnOpen(w io.Writer) {
	io.Copy(w, s.conn)
}

func (s *Server) OnMessage(data []byte) {
	s.conn.Write(data)
}

func (s *Server) OnClose() {
	s.conn.Close()
}

func NewServ(ip string, port int) *Server {
	return &Server{
		hostName: fmt.Sprintf("%s:%d", ip, port),
	}
}

type Client struct {
	hostName string
	sock     net.Conn
	tx       io.Writer
}

func (s *Client) Create() error {
	listener, err := net.Listen("tcp", s.hostName)
	if err != nil {
		return err
	}
	go func() {
		s.sock, err = listener.Accept()
		if err != nil {
			return
		}
	}()
	return nil
}

func (s *Client) OnOpen(w io.Writer) {
	io.Copy(w, s.sock)
}

func (s *Client) OnMessage(data []byte) {
	if _, err := s.sock.Write(data); err != nil {
		log.Printf("write data error: %s", err.Error())
	}
}

func (s *Client) OnClose() {
}

func NewCli(ip string, port int) *Client {
	return &Client{
		hostName: fmt.Sprintf("%s:%d", ip, port),
	}
}
