package model

type Message struct {
	Topic   string
	QOS     int
	Retain  bool
	Content []byte
}
