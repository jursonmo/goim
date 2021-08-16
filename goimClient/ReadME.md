
#### 问题：goim client 为了实现http发送数据到logic,也使用了discovery 来发现“goim.logic" 服务，但是logic 默认情况下注册服务时，注册的地址只有grpc://xxx:yyy ， 所有需要修改logic 注册服务时的代码：

```go
/goim/cmd/logic/main.go:

func register(dis *naming.Discovery, srv *logic.Logic) context.CancelFunc {
	env := conf.Conf.Env
	addr := ip.InternalIP()
	_, port, _ := net.SplitHostPort(conf.Conf.RPCServer.Addr)
	_, httpPort, _ := net.SplitHostPort(conf.Conf.HTTPServer.Addr)
	ins := &naming.Instance{
		Region:   env.Region,
		Zone:     env.Zone,
		Env:      env.DeployEnv,
		Hostname: env.Host,
		AppID:    appid,
		Addrs: []string{
			"grpc://" + addr + ":" + port,
			"http://" + addr + ":" + httpPort, //register http service for goim client
		},
		Metadata: map[string]string{
			model.MetaWeight: strconv.FormatInt(env.Weight, 10),
		},
    }
    ....
}
```
但是这样后，goim client 解释到 地址不是两个，而是把grpc 和http 的地址信息放在一起变成了一个
然后我又修改了：
```go
github.com/bilibili/discovery@v1.0.1/naming/client.go

// register Register an instance with discovery
func (d *Discovery) register(ctx context.Context, ins *Instance) (err error) {
	d.mutex.RLock()
	c := d.c
	d.mutex.RUnlock()
	.......
	params := d.newParams(c)
    params.Set("appid", ins.AppID)
    //modify 
	//params.Set("addrs", strings.Join(ins.Addrs, ","))	
	for _, addr := range ins.Addrs { //主要是做了这个修改
		params.Add("addrs", addr)
	}
	//modify end
	params.Set("version", ins.Version)
	params.Set("status", _statusUP)
	params.Set("metadata", string(metadata))
	if err = d.httpClient.Post(ctx, uri, "", params, &res); err != nil {
		d.switchNode()
		log.Errorf("discovery: register client.Get(%v)  zone(%s) env(%s) appid(%s) addrs(%v) error(%v)",
			uri, c.Zone, c.Env, ins.AppID, ins.Addrs, err)
		return
    }
    ....
}
```
但是goim client discovery 还是把两个地址合并成一个，导致grpc 连接都出错。

估计要升级下bilibili discovery