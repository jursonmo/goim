package goimClient

import (
	"bufio"
	"net"
	"time"

	pkgerr "github.com/pkg/errors"
)

type Transport struct {
	conn net.Conn
	rw   bufio.ReadWriter
	DialConf
	Conf interface{}
}

type DialConf struct {
	Scheme    string
	Host      string
	Local     string        //"1.1.1.1:8080"
	Timeout   time.Duration //Dial timeout
	KeepAlive time.Duration
}

func DefaultDialConf() DialConf {
	return DialConf{Timeout: time.Second * 2, KeepAlive: 10 * time.Minute}
}

func NewTransportConf(scheme string) (interface{}, error) {
	switch scheme {
	case "tcp":
		return DefaultTCPConf(), nil
	default:
		return nil, pkgerr.Errorf("NewTransportConf: unknow %s", scheme)
	}
	return nil, pkgerr.Errorf("NewTransportConf: unknow %s", scheme)
}
