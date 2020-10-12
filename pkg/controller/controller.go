package controller

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	log "github.com/sirupsen/logrus"
	"github.com/zhaohuabing/kube2zk/pkg/registry"
	"github.com/zhaohuabing/kube2zk/pkg/utils"

	api_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"encoding/json"
)

const (
	maxRetries = 5
	rpcService = "rpc-service"
)

type RPCServiceController struct {
	clientset     kubernetes.Interface
	queue         workqueue.RateLimitingInterface
	informer      cache.SharedIndexInformer
	registry      registry.Registry
	syncperiod    time.Duration
	registryMutex sync.Mutex
}

type PodEvent struct {
	key string
	pod *api_v1.Pod
}

// Start the controller, then waits for process termination signals
func (c *RPCServiceController) Start() {
	stopCh := make(chan struct{})
	defer close(stopCh)

	go c.run(stopCh)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	signal.Notify(sigterm, syscall.SIGINT)
	<-sigterm
}

func NewRPCServiceController(reg registry.Registry, namespace string, syncperiod time.Duration) *RPCServiceController {
	var kubeClient kubernetes.Interface

	if _, err := rest.InClusterConfig(); err != nil {
		kubeClient = utils.GetClientOutOfCluster()
	} else {
		kubeClient = utils.GetClient()
	}

	labelSelector := labels.Set(map[string]string{"rpc-service": "true"}).AsSelector()
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
				options.LabelSelector = labelSelector.String()
				return kubeClient.CoreV1().Pods(namespace).List(options)
			},
			WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = labelSelector.String()
				return kubeClient.CoreV1().Pods(namespace).Watch(options)
			},
		},
		&api_v1.Pod{},
		//we don't need resync here because we have another task periodically list all the pods and sync rpcservices to zk
		0,
		cache.Indexers{},
	)

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(&PodEvent{
					key: key,
					pod: obj.(*api_v1.Pod),
				})
			}
		},
		UpdateFunc: func(old, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(old)
			if err == nil {
				queue.Add(&PodEvent{
					key: key,
					pod: new.(*api_v1.Pod),
				})
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(&PodEvent{
					key: key,
					pod: obj.(*api_v1.Pod),
				})
			}
		},
	})

	return &RPCServiceController{
		registry:   reg,
		clientset:  kubeClient,
		informer:   informer,
		queue:      queue,
		syncperiod: syncperiod,
	}
}

// Run starts the RPCService controller
func (c *RPCServiceController) run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	log.Info("Starting RPCService controller")

	go c.informer.Run(stopCh)

	// wait until cache is synced
	if !cache.WaitForCacheSync(stopCh, c.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	log.Info("RPCService controller synced and ready")

	// sync all the services to registry at start and  periodically
	go wait.Until(c.batchSync, c.syncperiod, stopCh)
	wait.Until(c.runWorker, time.Second, stopCh)
}

func (c *RPCServiceController) HasSynced() bool {
	return c.informer.HasSynced()
}

func (c *RPCServiceController) LastSyncResourceVersion() string {
	return c.informer.LastSyncResourceVersion()
}

func (c *RPCServiceController) runWorker() {
	for c.processNextItem() {
		// continue looping
	}
}

func (c *RPCServiceController) processNextItem() bool {
	podEvent, quit := c.queue.Get()

	if quit {
		return false
	}
	defer c.queue.Done(podEvent)
	err := c.processItem(podEvent.(*PodEvent))
	if err == nil {
		// No error, reset the ratelimit counters
		c.queue.Forget(podEvent)
	} else if c.queue.NumRequeues(podEvent) < maxRetries {
		log.Errorf("Error processing %s (will retry): %v", podEvent, err)
		c.queue.AddRateLimited(podEvent)
	} else {
		// err != nil and too many retries
		log.Errorf("Error processing %s (giving up): %v", podEvent, err)
		c.queue.Forget(podEvent)
		utilruntime.HandleError(err)
	}

	return true
}

func (c *RPCServiceController) processItem(podEvent *PodEvent) error {
	c.registryMutex.Lock()
	defer c.registryMutex.Unlock()

	obj, exists, err := c.informer.GetIndexer().GetByKey(podEvent.key)

	if err != nil {
		return fmt.Errorf("error fetching object with key %s from store: %v", podEvent.key, err)
	}

	if !exists {
		log.Infof("rpc services on pod %s %s has been deleted.", podEvent.pod.Namespace, podEvent.pod.Name)
		services := parseServiceInfo(podEvent.pod)
		for _, service := range services {
			if err := c.registry.DeleteServiceInstance(service); err != nil {
				return err
			}
		}
		return nil
	}

	// Use the latest status retrieving from cache
	pod := obj.(*api_v1.Pod)

	log.Infof("rpc services on pod %s %s has been added or updated.", pod.Namespace, pod.Name)
	if pod.Name == "" || pod.Status.PodIP == "" {
		log.Printf("pod %s %s has neither name nor pod ip. skip this ADD notification.", pod.Namespace, pod.Name)
		return nil
	}

	services := parseServiceInfo(pod)
	for _, service := range services {
		if err := c.registry.AddOrUpdateServiceInstance(service); err != nil {
			return err
		}
	}
	return nil
}

// Sync all the rpcService
func (c *RPCServiceController) batchSync() {
	c.registryMutex.Lock()
	defer c.registryMutex.Unlock()

	podList := c.informer.GetIndexer().List()
	serviceList := map[string][]registry.RPCService{}
	var ok bool
	var instances []registry.RPCService

	for _, pod := range podList {
		rpcServices := parseServiceInfo(pod.(*api_v1.Pod))
		for _, service := range rpcServices {
			if instances, ok = serviceList[service.ServiceName]; !ok {
				instances = make([]registry.RPCService, 0, 8)
			}
			instances = append(instances, service)
			serviceList[service.ServiceName] = instances
		}
	}

	for k, v := range serviceList {
		err := c.registry.RefreshService(k, v)
		if err != nil {
			log.Errorf("failed to refresh service: %v", err)
		}
	}
}

func parseServiceInfo(pod *api_v1.Pod) []registry.RPCService {
	services := make([]registry.RPCService, 0)
	json.Unmarshal([]byte(pod.ObjectMeta.Annotations[rpcService]), &services)
	for i := range services {
		services[i].Address = pod.Status.PodIP
	}
	return services
}
