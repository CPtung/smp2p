package ssh

import (
	"io"
	"log"
)

type SSHServ struct {
}

func (s *SSHServ) Create() error {
	return nil
}

func (s *SSHServ) OnOpen(w io.Writer) {

}

func (s *SSHServ) OnMessage([]byte) {

}

func (s *SSHServ) OnClose() {
	log.Println("on service close")
}

func NewServ() *SSHServ {
	return &SSHServ{}
}

type SSHCli struct {
}

func (s *SSHCli) Create() error {
	return nil
}

func (s *SSHCli) OnOpen(w io.Writer) {

}

func (s *SSHCli) OnMessage([]byte) {

}

func (s *SSHCli) OnClose() {
	log.Println("on service close")
}

func NewCli() *SSHCli {
	return &SSHCli{}
}
