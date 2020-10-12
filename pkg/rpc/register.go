package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path"
	"strings"
	"time"

	"github.com/go-zookeeper/zk"
)

type OptType int

const (
	OptDelete OptType = iota
	OptAdd
	OptSet
	OptCreate
)

type rpcConfig struct {
	operator OptType `json:"-"`
	service  string  `json:"-"`
	version  int32   `json:"-"`
	changed  bool    `json:"-"`

	Config  []string `json:"config"`
	Servers []string `json:"servers"`
}

type Register struct {
	Servers  []string
	BasePath string
	QPS      float64 // Qps用于限制写入zookeeper的速度

	conn     *zk.Conn
	inited   bool
	throttle <-chan time.Time
}

func (reg *Register) Init() error {
	if reg.inited {
		return nil
	}

	if len(reg.Servers) <= 0 {
		return errors.New("Register.Servers can not be empty")
	}

	conn, _, err := zk.Connect(reg.Servers, 30*time.Second)
	if err != nil {
		return err
	}

	reg.conn = conn
	if reg.BasePath == "" {
		reg.BasePath = "/rpc_v2"
	}

	//create the basePath node if it doesn't exist
	_, _, err = reg.conn.Get(reg.BasePath)
	if err == zk.ErrNoNode {
		acls := zk.WorldACL(zk.PermAll)
		if _, err := reg.conn.Create(reg.BasePath, nil, 0, acls); err != nil {
			return err
		}
	}

	if reg.QPS == 0 {
		reg.QPS = 10.0
	}

	if reg.QPS > 0 {
		reg.throttle = time.Tick(time.Duration(1e6/(reg.QPS)) * time.Microsecond)
	}

	reg.inited = true

	return nil
}

// Update 函数一般在程序初始化时调用，或者在定时器中调用。
// 作用是强制刷新指定service注册的所有节点IP，防止丢失Pod变更事件，导致
// 已服务的节点信息没有及时刷新。
func (reg *Register) Update(service string, servers []string) error {
	if len(servers) == 0 {
		return errors.New("servers can not be empty")
	}

	cfg, err := reg.getConfig(service)
	if err != nil {
		if err == zk.ErrNoNode {
			cfg = &rpcConfig{
				operator: OptCreate,
				service:  service,
				Config:   []string{"zone kg 8M", "kg_round_robin broken_tries=2 health_checks=5 max_uri_slots=0", "keepalive 32"},
			}
		} else {
			return err
		}
	} else {
		cfg.operator = OptSet
	}

	if cfg.operator == OptCreate || diff(cfg.Servers, servers) {
		cfg.Servers = servers
		cfg.changed = true

		for {
			err = reg.send(cfg)
			if err == nil {
				break
			}

			if err == zk.ErrNodeExists {
				cfg.operator = OptSet
				continue
			}

			if err == zk.ErrNoNode {
				cfg.operator = OptCreate
				continue
			}

			if err == zk.ErrBadVersion {
				err = reg.Update(service, servers)
			}

			return err
		}
	}

	return nil
}

// Add 函数负责将新的节点注册到指定service下
func (reg *Register) Add(service, addr string) error {
	cfg, err := reg.getConfig(service)
	if err != nil {
		if err == zk.ErrNoNode {
			cfg = &rpcConfig{
				operator: OptCreate,
				service:  service,
				Config:   []string{"zone kg 8M", "kg_round_robin broken_tries=2 health_checks=5 max_uri_slots=0", "keepalive 32"},
				Servers:  []string{addr},
			}
		} else {
			return err
		}
	} else {
		cfg.operator = OptAdd
	}

	if cfg.operator != OptCreate {
		cfg = reg.mergeConfig(cfg, addr)
		cfg.operator = OptSet
	}

	for {
		err = reg.send(cfg)
		if err == nil {
			break
		}

		if err == zk.ErrNodeExists {
			cfg.operator = OptSet
			continue
		}

		if err == zk.ErrNoNode {
			cfg.operator = OptCreate
			continue
		}

		if err == zk.ErrBadVersion {
			err = reg.Add(service, addr)
		}

		return err
	}

	return nil
}

// Delete 函数负责从指定service删除节点
func (reg *Register) Delete(service, addr string) error {
	cfg, err := reg.getConfig(service)
	if err != nil {
		if err == zk.ErrNoNode {
			return nil
		}
		return err
	} else {
		cfg.operator = OptDelete
	}

	cfg = reg.mergeConfig(cfg, addr)
	if len(cfg.Servers) > 0 {
		cfg.operator = OptSet
	}

	for {
		err = reg.send(cfg)
		if err == nil {
			break
		}

		if err == zk.ErrNoNode {
			return nil
		}

		if err == zk.ErrBadVersion {
			err = reg.Delete(service, addr)
		}

		return err
	}

	return nil
}

func (reg *Register) getConfig(service string) (*rpcConfig, error) {
	p := path.Join(reg.BasePath, service)

	b, stat, err := reg.conn.Get(p)
	if err != nil {
		return nil, err
	}

	cfg := new(rpcConfig)
	err = json.Unmarshal(b, cfg)
	if err != nil {
		return nil, fmt.Errorf("get invalid json, error: %v", err)
	}

	cfg.version = stat.Version
	cfg.service = service
	return cfg, nil
}

func (reg *Register) mergeConfig(cfg *rpcConfig, newAddress string) *rpcConfig {
	var servers = cfg.Servers
	newServers := make([]string, 0, len(servers)+1)
	found := false

	newAddress = strings.TrimSpace(newAddress)
	for _, srv := range servers {
		srv = strings.TrimSpace(srv)

		if srv == newAddress {
			if cfg.operator == OptDelete {
				cfg.changed = true
				continue
			}

			if cfg.operator == OptAdd {
				found = true
			}
		}

		newServers = append(newServers, srv)
	}

	if cfg.operator == OptAdd && !found {
		newServers = append(newServers, newAddress)
		cfg.changed = true
	}

	cfg.Servers = newServers
	return cfg
}

func (reg *Register) send(cfg *rpcConfig) error {
	if cfg.operator == OptSet && !cfg.changed {
		log.Printf("service: %s, no changed.\n", cfg.service)
		return nil
	}

	p := path.Join(reg.BasePath, cfg.service)
	if cfg.operator == OptDelete && len(cfg.Servers) == 0 {
		<-reg.throttle
		err := reg.conn.Delete(p, cfg.version)
		return err
	}

	b, _ := json.Marshal(cfg)
	acls := zk.WorldACL(zk.PermAll)

	if cfg.operator == OptCreate {
		<-reg.throttle
		_, err := reg.conn.Create(p, b, 0, acls)
		return err
	}

	if cfg.operator == OptSet {
		<-reg.throttle
		_, err := reg.conn.Set(p, b, cfg.version)
		return err
	}

	return nil
}
