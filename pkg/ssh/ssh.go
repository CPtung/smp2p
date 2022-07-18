package ssh

import (
	"fmt"
	"io"
	"log"
	"net"
)

type TcpServer interface {
	Bind() error
	Tx(io.Writer) error
	Rx([]byte) (int, error)
	Close()
}

type SshServer struct {
	hostName string
	conn     net.Conn
}

func NewServer(ip string, port int) TcpServer {
	return &SshServer{
		hostName: fmt.Sprintf("%s:%d", ip, port),
	}
}

func (s *SshServer) Bind() (err error) {
	if s.conn, err = net.Dial("tcp", s.hostName); err != nil {
		return err
	}
	return nil
}

func (s *SshServer) Tx(w io.Writer) error {
	_, err := io.Copy(w, s.conn)
	if err != nil {
		log.Printf("server tx error: %s", err.Error())
	}
	return nil
}

func (s *SshServer) Rx(data []byte) (int, error) {
	return s.conn.Write(data)
}

func (s *SshServer) Close() {
	s.conn.Close()
}
