package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/zhaohuabing/kube2zk/pkg/controller"
	"github.com/zhaohuabing/kube2zk/pkg/registry"
	"github.com/zhaohuabing/kube2zk/pkg/rpc"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
}

func main() {
	namespace := flag.String("namespace", meta_v1.NamespaceAll, "which namespace to watch, kube2zk will watch all namespaces if this parameter is not specified")
	mockregistry := flag.Bool("mockregistry", false, "connect to Mock Registry, default is false")
	zkservers := flag.String("zkservers", "127.0.0.1:2181", "zookeeper servers address")
	zkpath := flag.String("zkpath", "/rpc_v2", "base path of rpc service on zookeeper")
	syncperiod := flag.Duration("syncperiod", time.Hour, "the period to sync registry with k8s")
	flag.Parse()

	if !*mockregistry {
		if *zkservers == "" {
			fmt.Println("Invalid zookeeper address")
			return
		}

		if *zkpath == "" {
			fmt.Println("Invalid zookeeper base path")
			return
		}
	}

	var reg registry.Registry
	if *mockregistry {
		reg = &registry.MockRegistry{}
	} else {
		servers := strings.Split(*zkservers, ",")
		client := rpc.Register{
			Servers:  servers,
			BasePath: *zkpath,
		}
		client.Init()
		reg = &registry.ZKRegistry{
			ZKClient: client,
		}
	}

	controller := controller.NewRPCServiceController(reg, *namespace, *syncperiod)
	controller.Start()
}
