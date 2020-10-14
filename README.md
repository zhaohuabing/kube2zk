kube2zk watches k8s pod and register the services defined in pod annotation to zoookeeper.

kube2zk 将 Pod 中的自定义服务注册到 zookeeper 中。

# 工作原理：

* kube2zk 提供了一个 controller，该 controller 会 watch k8s 集群中的 pod 事件（添加、变化、删除），并将 pod 中使用 annotation 标注的 RPC Service 注册到 zookeeper 的指定路径下。
* 除了 watch pod 事件外，kube2zk 还会定期将 k8s 集群中的 PRC Service 批量同步到 zookeeper 中，以避免可能有消息丢失导致的数据不一致，缺省同步时间间隔为 1 小时。

# 使用说明：

## 二进制

```bash
Usage of ./out/kube2zk:
  -mockregistry
        connect to Mock Registry, default is false
  -namespace string
        which namespace to watch, kube2zk will watch all namespaces if this parameter is not specified
  -syncperiod duration
        the period to sync registry with k8s (default 1h0m0s)
  -zkpath string
        base path of rpc service on zookeeper (default "/rpc_v2")
  -zkservers string
        zookeeper servers address (default "127.0.0.1:2181")
```

## k8s

```bash
cd kube2zk/k8s/
kubectl apply -f zookeeper.yaml
kubectl apply -f kube2zk.yaml
kubectl apply -f test-rpc-service.yaml
```

如果缺省参数不满足要求，可以在 kube2zk.yaml 中可以通过环境变量修改：

```yaml

......
          env:
            - name: namespace
              value: ""
            - name: zkservers
              value: "zookeeper:2181"
            - name: zkpath
              value: "/rpc_v2"
```

验证 zookeeper 中注册的服务：

```bash
# 进入 zk 容器，请将 zookeeper-8598999f77-lrvd4 替换为你环境中的 pod name
k exec -it zookeeper-8598999f77-lrvd4 bash

# 进入容器后执行 zk 客户端命令行
./bin/zkCli.sh

# 验证注册的服务
ls /rpc_v2
[test-rpc-server-1, test-rpc-server-2]

# 查看服务实例信息
get /rpc_v2/test-rpc-server-1
{"config":["zone kg 8M","kg_round_robin broken_tries=2 health_checks=5 max_uri_slots=0","keepalive 32"],"servers":["172.16.0.251","172.16.1.46","172.16.1.44","172.16.1.45","172.16.0.67"]}
cZxid = 0x21
ctime = Sat Oct 10 02:29:25 UTC 2020
mZxid = 0x29
mtime = Sat Oct 10 02:29:26 UTC 2020
pZxid = 0x21
cversion = 0
dataVersion = 4
aclVersion = 0
ephemeralOwner = 0x0
dataLength = 187
numChildren = 0
```

# 服务定义

* 使用 label rpc-service: "true" 表示该 Pod 中提供了自定义服务
* 使用 annotation rpc-service 定义 RPC Service，一个 Pod 中可以定义一到多个 RPC Service

举例说明
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-service
  labels:
    app: test-service
spec:
  selector:
    matchLabels:
      app: test-service
  replicas: 2
  template:
    metadata:
      labels:
        app: test-service
        rpc-service: "true"  #声明 pod 支持 rpc service
      annotations:
        rpc-service: '[{"serviceName": "test-rpc-server-1", "version": "v0" }, { "serviceName": "test-rpc-server-2", "version": "v1"}]' # 定义 pod 中提供的 rpc service。备注：目前 zookeeper 中没有使用此处定义的 version 字段，后续有需要可以使用，也可以扩展其他字段，例如 LB 算法等。
    spec:
      containers:
        - name: simple-http-server
          image: zhaohuabing/simple-http-server
          env:
            - name: PYTHONUNBUFFERED
              value: "1"
          ports:
            - containerPort: 5000
```
