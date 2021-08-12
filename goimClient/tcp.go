package goimClient

import (
	"net"

	log "github.com/golang/glog"
	pkgerr "github.com/pkg/errors"
)

type TCPConf struct {
	Sndbuf       int
	Rcvbuf       int
	KeepAlive    bool
	Reader       int
	ReadBuf      int
	ReadBufSize  int
	Writer       int
	WriteBuf     int
	WriteBufSize int
}

func DefaultTCPConf() *TCPConf {
	return &TCPConf{
		Sndbuf:       4096,
		Rcvbuf:       4096,
		KeepAlive:    false,
		Reader:       32,
		ReadBuf:      1024,
		ReadBufSize:  8192,
		Writer:       32,
		WriteBuf:     1024,
		WriteBufSize: 8192,
	}
}

func (t *Transport) TCPDial() (net.Conn, error) {
	dc := &t.DialConf
	localAddr, err := net.ResolveTCPAddr("tcp4", t.Local)
	if err != nil {
		return nil, pkgerr.Wrap(err, "ResolveTCPAddr")
	}
	dialer := net.Dialer{
		LocalAddr: localAddr,
		KeepAlive: dc.KeepAlive,
		Timeout:   dc.Timeout,
	}
	conn, err := dialer.Dial(dc.Scheme, dc.Host)
	if err != nil {
		return nil, pkgerr.Wrap(err, "TCPDial")
	}
	return conn, err
}

func SetConnConf(conn net.Conn, c *TCPConf) {
	TCPConn := conn.(*net.TCPConn)
	var err error
	if err = TCPConn.SetKeepAlive(c.KeepAlive); err != nil {
		log.Errorf("conn.SetKeepAlive() error(%v)", err)
		return
	}
	if err = TCPConn.SetReadBuffer(c.Rcvbuf); err != nil {
		log.Errorf("conn.SetReadBuffer() error(%v)", err)
		return
	}
	if err = TCPConn.SetWriteBuffer(c.Sndbuf); err != nil {
		log.Errorf("conn.SetWriteBuffer() error(%v)", err)
		return
	}
}
