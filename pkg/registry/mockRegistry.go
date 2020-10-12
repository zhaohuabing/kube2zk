package registry

import (
	log "github.com/sirupsen/logrus"
)

type MockRegistry struct {
}

func (m *MockRegistry) RefreshService(name string, instances []RPCService) error {
	log.Infof("MockRegistry: refresh service : %s", name)
	return nil
}

func (m *MockRegistry) AddOrUpdateServiceInstance(service RPCService) error {
	log.Infof("MockRegistry: add or update service : %s %s", service.ServiceName, service.Address)
	return nil
}

func (m *MockRegistry) DeleteServiceInstance(service RPCService) error {
	log.Infof("MockRegistry: delete service : %s %s", service.ServiceName, service.Address)
	return nil
}
