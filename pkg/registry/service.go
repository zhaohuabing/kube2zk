package registry

type RPCService struct {
	ServiceName string `json:"serviceName"`
	Version     string `json:"version"`
	Address     string
}
