package goimClient

import (
	"bufio"
	"net"
	"time"

	xtime "github.com/Terry-Mao/goim/pkg/time"
	"github.com/bilibili/discovery/naming"
	pkgerr "github.com/pkg/errors"
)

type Option func(c *Client)

func WithAccepts(accepts []int32) Option {
	return func(c *Client) {
		c.Accepts = accepts
	}
}
func WithPlatform(platform string) Option {
	return func(c *Client) {
		c.Platform = platform
	}
}

func WithRegion(v string) Option {
	return func(c *Client) {
		if v != "" {
			c.disConf.Region = v
		}
	}
}
func WithZone(v string) Option {
	return func(c *Client) {
		if v != "" {
			c.disConf.Zone = v
		}
	}
}
func WithEnv(v string) Option {
	return func(c *Client) {
		if v != "" {
			c.disConf.Env = v
		}
	}
}

type Transport struct {
	conn net.Conn
	rw   bufio.ReadWriter
	DialConf
	Conf interface{}
}

type Discovery struct {
	naming.Config
}

// net.ipv4.tcp_keepalive_time=7200
// net.ipv4.tcp_keepalive_intvl=75
// net.ipv4.tcp_keepalive_probes=9
type HeartBeatConf struct {
	KeepaliveTime   xtime.Duration
	KeepaliveIntvl  xtime.Duration
	KeepaliveProbes int
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

func DefaultDiscoveryConf() *naming.Config {
	return &naming.Config{
		Nodes:  []string{"127.0.0.1:7171"},
		Region: "sh",
		Zone:   "sh001",
		Env:    "dev",
		Host:   "client01",
	}
}

func (c *Discovery) Options() []Option {
	return []Option{
		WithRegion(c.Region),
		WithZone(c.Zone),
		WithEnv(c.Env),
	}
}

func DefaultHeartBeatConf() *HeartBeatConf {
	return &HeartBeatConf{
		KeepaliveTime:   xtime.Duration(10 * time.Second),
		KeepaliveIntvl:  xtime.Duration(time.Second),
		KeepaliveProbes: 3,
	}
}
