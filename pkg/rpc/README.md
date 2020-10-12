## Example


```go

//新建Register
reg := &rpc.Register{
    Servers:       []string{"127.0.0.1:2181"},
    BasePath:      "/rpc_v2",
    FlushDuration: 5 * time.Second,
}

//启动
reg.Run()

//注册节点
cb = func(operator rpc.OptType, service, address string, err error) {
    //此回调函数会在注册出错时调用
    //do something
}
AddServer(service, fmt.Sprintf("%s:%d", podIP, port), cb)

//删除节点
cb2 = func(operator rpc.OptType, service, address string, err error) {
    //此回调函数会在注册出错时调用
    //do something
}
DelServer(service, fmt.Sprintf("%s:%d", podIP, port), cb2)
```