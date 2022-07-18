package ssh

import (
	"fmt"
	"io"
	"log"
	"net"

	"github.com/CPtung/smp2p/pkg/signaling"
)

type TcpConn interface {
	Tx() io.Reader
	Rx([]byte) (int, error)
	Close()
}

type TcpClient interface {
	Bind() error
	Listen(func(TcpConn) signaling.Offer)
}

type SshClient struct {
	hostName string
	listener net.Listener
	tx       io.Writer
}

type SshConn struct {
	conn net.Conn
}

func NewClient(ip string, port int) TcpClient {
	return &SshClient{
		hostName: fmt.Sprintf("%s:%d", ip, port),
	}
}

func (s *SshClient) Bind() (err error) {
	s.listener, err = net.Listen("tcp", s.hostName)
	if err != nil {
		return err
	}
	return nil
}

func (s *SshClient) Listen(handle func(TcpConn) signaling.Offer) {
	go func() {
		for {
			sock, err := s.listener.Accept()
			if err != nil {
				log.Println(err)
				continue
			}
			handle(newSshConn(sock))
		}
	}()
}

func newSshConn(conn net.Conn) *SshConn {
	return &SshConn{conn: conn}
}

func (s *SshConn) Tx() io.Reader {
	return s.conn
}

func (s *SshConn) Rx(data []byte) (int, error) {
	return s.conn.Write(data)
}

func (s *SshConn) Close() {
	s.conn.Close()
}
