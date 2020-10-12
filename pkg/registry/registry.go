package registry

type Registry interface {

	// refresh a service, all the current instances will be deleted and be replaced with the instances specified in the parameters
	RefreshService(service string, instances []RPCService) error

	// add it if the service doesn't exist, or update it if it's already there
	AddOrUpdateServiceInstance(service RPCService) error

	// delete a service
	DeleteServiceInstance(service RPCService) error
}
