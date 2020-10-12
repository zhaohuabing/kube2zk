package registry

import (
	"github.com/zhaohuabing/kube2zk/pkg/rpc"
	log "github.com/sirupsen/logrus"
)

type ZKRegistry struct {
	ZKClient rpc.Register
}

func (m *ZKRegistry) RefreshService(name string, instances []RPCService) error {
	log.Infof("ZKRegistry: refresh service : %s", name)

	// we only need the IP address of service instance at this moment
	addresses := make([]string, len(instances))
	for i, instance := range instances {
		addresses[i] = instance.Address
	}
	return m.ZKClient.Update(name, addresses)
}

func (m *ZKRegistry) AddOrUpdateServiceInstance(service RPCService) error {
	log.Infof("ZKRegistry: add or update service : %s %s", service.ServiceName, service.Address)
	return m.ZKClient.Add(service.ServiceName, service.Address)
}

func (m *ZKRegistry) DeleteServiceInstance(service RPCService) error {
	log.Infof("ZKRegistry: delete service : %s %s", service.ServiceName, service.Address)
	return m.ZKClient.Delete(service.ServiceName, service.Address)
}
