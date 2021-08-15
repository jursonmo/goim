package goimClient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Terry-Mao/goim/api/protocol"
	"github.com/bilibili/discovery/naming"
	pkgerr "github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	log "github.com/golang/glog"
)

type Client struct {
	closed int32
	//bufAllocator //todo
	output        chan []byte
	heartbeatChan chan []byte
	cancel        context.CancelFunc
	recvCb        func([]byte)
	//wg            WaitGroupWrapper
	eg *errgroup.Group
	Transport
	MsgAttr
	Token
	//discovery Configure
	disConf *naming.Config
	logics  map[string]*logic //key is logic hostname
}

//get logic addr to send message
type logic struct {
	addr string
	hc   *http.Client
}

type bufAllocator interface {
	Alloc() []byte
}

type WSConf struct {
	//todo:
}

type MsgAttr struct {
	Ver int32
	Seq int32
}
type Token struct {
	Mid      int64   `json:"mid"` //
	Key      string  `json:"key"` //
	RoomID   string  `json:"room_id"`
	Platform string  `json:"platform"`
	Accepts  []int32 `json:"accepts"`
}

//mid 相当于用户id
//Key: 相当于一个连接标识， 一个mid 可以有多个连接，每个连接标识不一样就行，这样其他用户给这个用户发消息时，多个连接都能收到，比如多终端场景。
// RoomID: 表示加入那个群里，相当于订阅了这个群的消息
//Accepts: 标识这个Key 连接接受哪些类型的消息，也就消息从job发到comet时，comet 在发送给client 时，就会检查消息的类型。

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
func (c *Client) String() string {
	return fmt.Sprintf("mid:%d, key:%s, roomid:%s", c.Mid, c.Key, c.RoomID)
}

// comet addr
func NewClient(addr string, mid int64, key, roomID string, ops ...Option) (*Client, error) {
	var err error

	dc := DefaultDialConf()
	dc.Scheme, dc.Host, err = ParseAddr(addr)
	if err != nil {
		return nil, err
	}
	c := new(Client)
	c.Transport.DialConf = dc
	c.Transport.Conf, err = NewTransportConf(dc.Scheme)
	if err != nil {
		return nil, err
	}

	c.Mid = mid
	c.Key = key
	c.RoomID = roomID
	for _, op := range ops {
		op(c)
	}

	return c, nil
}

//token 一般都是从另外的服务获取，然后根据token 创建Client
func NewClientWithToken(addr string, t Token, ops ...Option) (*Client, error) {
	var err error

	dc := DefaultDialConf()
	dc.Scheme, dc.Host, err = ParseAddr(addr)
	if err != nil {
		return nil, err
	}
	c := new(Client)
	c.Transport.DialConf = dc
	c.Transport.Conf, err = NewTransportConf(dc.Scheme)
	if err != nil {
		return nil, err
	}

	c.Token = t
	for _, op := range ops {
		op(c)
	}
	return c, nil
}

func ParseAddr(addr string) (scheme string, host string, err error) {
	network, err := url.Parse(addr)
	if err != nil {
		return "", "", err
	}
	return network.Scheme, network.Host, err
}

func (c *Client) Start() error {
	// init conn
	err := (&c.Transport).Dial()
	if err != nil {
		return err
	}

	//auth
	err = c.Auth()
	if err != nil {
		return err
	}

	rootCtx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	//start working
	//c.wg.AddRun(func() { c.Output(rootCtx) })
	//c.wg.AddRun(func() { c.heartbeat(rootCtx) })
	//c.wg.AddRun(func() { c.Inbound(rootCtx) })

	var taskCtx context.Context
	c.eg, taskCtx = errgroup.WithContext(rootCtx)
	c.eg.Go(func() error { return c.Output(taskCtx) })
	c.eg.Go(func() error { return c.heartbeat(taskCtx) })
	//c.eg.Go(func() error { return c.watchLogic(taskCtx) })
	return nil
}

func (c *Client) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return pkgerr.Errorf("client has closed?(%d)", c.closed)
	}
	c.cancel() //cancel Output and heartbeat, 如果
	c.eg.Wait()
	return nil
}

func (c *Client) Closed() bool {
	return atomic.LoadInt32(&c.closed) == 1
}

func (t *Transport) Dial() error {
	var conn net.Conn
	var err error

	dc := t.DialConf
	switch dc.Scheme {
	case "tcp":
		conf, ok := t.Conf.(*TCPConf)
		if !ok {
			return pkgerr.Errorf("client Conf is not TCPConf")
		}
		conn, err = t.TCPDial()
		if err != nil {
			return err
		}
		SetConnConf(conn, conf)
	case "websocket":
		//todo

	default:
		return pkgerr.Errorf("unsuport scheme:%s", dc.Scheme)
	}
	t.conn = conn
	t.rw = bufio.ReadWriter{Reader: bufio.NewReaderSize(conn, 8096), Writer: bufio.NewWriterSize(conn, 8096)}
	return nil
}

func (c *Client) Auth() error {
	b, err := json.Marshal(&c.Token)
	if err != nil {
		return pkgerr.Wrap(err, "Auth Marshal token fail")
	}
	p := protocol.Proto{}
	p.Ver = c.Ver
	p.Op = protocol.OpAuth
	p.Seq = atomic.AddInt32(&c.Seq, 1)
	p.Body = b

	data := make([]byte, p.PktLen())
	p.Encode(data)
	c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err = c.conn.Write(data)
	c.conn.SetWriteDeadline(time.Time{})
	if err != nil {
		return pkgerr.Wrap(err, "send auth fail")
	}

	c.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err = p.Decode(c.conn)
	//err = p.Decode(c.rw)
	c.conn.SetReadDeadline(time.Time{})
	if err != nil {
		return pkgerr.Wrap(err, "read auth reply fail")
	}
	if p.Op != protocol.OpAuthReply {
		return pkgerr.Errorf("fail auth reply op:%d", p.Op)
	}
	return nil
}

func (c *Client) Output(ctx context.Context) (err error) {
	if c.output == nil {
		c.output = make(chan []byte, 128)
	}
	defer c.conn.Close() //maybe can make RecevieMsg or read quit
	defer func() {
		log.Errorf("Output task quit, client:%s, err:%v", c.String(), err)
	}()
	for {
		select {
		case <-ctx.Done():
			err = pkgerr.Wrap(ctx.Err(), " Output task canceled")
			return
		case data := <-c.output:
			_, err = c.rw.Write(data)
			if err != nil {
				return err
			}
			if len(c.output) == 0 && c.rw.Writer.Buffered() > 0 {
				c.rw.Writer.Flush()
			}
		}
	}
	return nil
}

func (c *Client) heartbeat(ctx context.Context) error {
	t := time.NewTicker(time.Second * 10)
	defer t.Stop()
	defer c.conn.Close()                                       //maybe can make write and read quit
	defer c.conn.SetDeadline(time.Now().Add(time.Millisecond)) //make RecevieMsg or read quit
	for {
		select {
		case <-ctx.Done():
			return pkgerr.Wrap(ctx.Err(), " heartbeat has been canceled")
		case <-t.C:
			p := protocol.Proto{}
			p.Ver = c.Ver
			p.Op = protocol.OpHeartbeat
			p.Seq = atomic.AddInt32(&c.Seq, 1)

			buf := make([]byte, p.PktLen())
			p.Encode(buf)
			c.output <- buf
			//
		}
	}
}

// RecevieMsg
func (c *Client) Input(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return pkgerr.Wrap(ctx.Err(), " ReceiveMsg has been canceled")
		default:
			//todo, set conn read
			p := protocol.Proto{}
			_, err := p.Decode(c.rw)
			if err != nil {
				return err
			}
			if p.Op == protocol.OpHeartbeatReply {
				//todo:
				continue
			}
			//todo: check p.Op in c.Accepts or check p.Ver if ok, p.Body 就原始发送消息了
			if p.Body != nil && c.recvCb != nil {
				c.recvCb(p.Body)
			}
		}
	}
	return nil
}

func (c *Client) watchLogic(ctx context.Context) error {
	conf := c.disConf
	dis := naming.New(conf)
	resolver := dis.Build("goim.comet")
	event := resolver.Watch()

	for {
		select {
		case <-ctx.Done():
			return pkgerr.Wrap(ctx.Err(), " watchLogic has been canceled")
		case _, ok := <-event:
			if !ok {
				return pkgerr.New("watchLogic exit")
			}
			ins, ok := resolver.Fetch()
			if ok {
				if err := c.newAddress(ins.Instances); err != nil {
					log.Errorf("watchLogic newAddress(%+v) error(%+v)", ins, err)
					continue
				}
				log.Infof("watchLogic change newAddress:%+v", ins)
			}
		}
	}
	return nil
}

func (c *Client) newAddress(insMap map[string][]*naming.Instance) error {
	ins := insMap[c.disConf.Zone]
	if len(ins) == 0 {
		return fmt.Errorf("watchComet instance is empty in zone:%s", c.disConf.Zone)
	}
	logics := make(map[string]*logic)
	for _, in := range ins {
		for _, addr := range in.Addrs {
			if !strings.Contains(addr, "http:") {
				continue
			}
			logics[in.Hostname] = &logic{addr: addr}
		}
	}
	if len(logics) > 0 {
		c.logics = logics
	}
	return nil
}

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (c *Client) SendMidMsgToServer(server string, mids []int64, op int32, msg string) (err error) {
	Url, err := url.Parse(fmt.Sprintf("http://%s/goim/push/mids", server))
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Add("operation", fmt.Sprintf("%d", op))
	for _, mid := range mids {
		params.Add("mids", fmt.Sprintf("%d", mid))
	}
	Url.RawQuery = params.Encode()
	//log.Infof("Url.String():%s", Url.String()) //output: http://127.0.0.1:3111/goim/push/mids?mids=1&operation=1000
	req, err := http.NewRequest("POST", Url.String(), strings.NewReader(msg))
	if err != nil {
		return pkgerr.Wrap(err, "NewRequest err")
	}

	hc := &http.Client{}
	return httpRequest(hc, req)
}

func (c *Client) SendMidMsg(mids []int64, op int32, msg string) (err error) {
	var hc *http.Client
	if len(c.logics) == 0 {
		return pkgerr.New("there is no msg server to send msg")
	}

	params := url.Values{}
	for _, mid := range mids {
		params.Add("mids", fmt.Sprintf("%d", mid))
	}
	params.Add("operation", fmt.Sprintf("%d", op))
	url_params := params.Encode()
	//url_params := fmt.Sprintf("mid=%d&operation=%d", mid, op)

	for _, logic := range c.logics {
		if logic.hc == nil {
			logic.hc = &http.Client{}
		}
		hc = logic.hc
		url := fmt.Sprintf("http://%s/goim/push/mids?%s", logic.addr, url_params)
		req, err := http.NewRequest("Post", url, strings.NewReader(msg))
		if err != nil {
			logic.hc = nil
			continue
		}
		err = httpRequest(hc, req)
		if err != nil {
			logic.hc = nil
			continue
		}
	}
	return nil
}
func httpRequest(hc *http.Client, req *http.Request) error {
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		response := &Response{}
		// de := json.NewDecoder(resp.Body)
		// err = de.Decode(response)
		body, err := ioutil.ReadAll(resp.Body) // read all data
		if err != nil {
			return err
		}
		err = json.Unmarshal(body, response)
		if err != nil {
			return err
		}
		//server result ok
		if response.Code != 0 {
			return fmt.Errorf("response.Code:%d", response.Code)
		}
	} else {
		io.Copy(ioutil.Discard, resp.Body) //drain all data for reuse tcp
		return fmt.Errorf("resp.StatusCode:%d", resp.StatusCode)
	}
	return nil
}

//io.Reader interface
func (c *Client) Read(b []byte) (n int, err error) {
	for {
		p := protocol.Proto{}
		_, err = p.Decode(c.rw)
		if err != nil {
			return
		}
		if p.Op == protocol.OpHeartbeatReply {
			log.Infof("OpHeartbeatReply:%d\n", p.Op)
			continue
		}
		log.Infof("Op:%d\n", p.Op)
		if p.Body != nil {
			n = copy(b, p.Body)
			return
		}
	}
	return
}
