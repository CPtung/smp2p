package service

import "io"

type Service interface {
	Create() error
	OnOpen(io.Writer)
	OnMessage([]byte)
	OnClose()
}
