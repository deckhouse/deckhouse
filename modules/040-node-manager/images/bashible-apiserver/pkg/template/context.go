package template

import (
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/yaml"
)

func NewContext(factory informers.SharedInformerFactory, configMapName, configMapKey string, updateHandler UpdateHandler) *Context {
	c := Context{
		configMapName: configMapName,
		configMapKey:  configMapKey,
		updateHandler: updateHandler,
	}

	c.subscribe(factory)

	return &c
}

type UpdateHandler interface {
	OnUpdate()
}

// Context manages bashible template context
type Context struct {
	rw sync.RWMutex

	// configMapKey in configmap to parse
	configMapName string
	configMapKey  string
	hasSynced     bool

	updateHandler UpdateHandler

	// data (taken by configMapKey from configmap) maps `contextKey` to `contextValue`,
	// the being arbitrary data for a combination of os, nodegroup, & kubeversion
	data map[string]interface{}
}

func (c *Context) subscribe(factory informers.SharedInformerFactory) chan struct{} {
	ch := make(chan map[string]string)
	stopInformer := make(chan struct{})

	// Launch the informer
	informer := factory.Core().V1().ConfigMaps().Informer()
	go informer.Run(stopInformer)

	// Subscribe to updates
	informer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: configMapFilter(c.configMapName),
		Handler:    &configMapEventHandler{ch},
	})

	// Store updates
	stopUpdater := make(chan struct{})
	go func() {
		for {
			select {
			case configMapData := <-ch:
				c.update(configMapData)
			case <-stopUpdater:
				close(stopInformer)
				return
			}
		}
	}()

	// Wait for the first sync of the informer cache, should not take long
	for !informer.HasSynced() {
		time.Sleep(200 * time.Millisecond)
	}

	return stopUpdater
}

func (c *Context) update(configMapData map[string]string) {
	c.rw.Lock()
	defer c.rw.Unlock()

	value, ok := configMapData[c.configMapKey]
	if !ok {
		// server error, so we panic
		panic(fmt.Sprintf("absent key \"%s\" in configmap %s\n", c.configMapKey, c.configMapName))
	}

	yaml.Unmarshal([]byte(value), &c.data)

	c.updateHandler.OnUpdate()

}

// Get retrieves a copy of context for the given configMapKey.
//
// TODO In future, node group name will be passed instead of a configMapKey.
func (c *Context) Get(contextKey string) (map[string]interface{}, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	raw, ok := c.data[contextKey]
	if !ok {
		return nil, fmt.Errorf("context not found for configMapKey \"%s\"", contextKey)
	}

	converted, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("cannot convert context for configMapKey \"%s\" to map[string]interface{}", contextKey)
	}

	copied := make(map[string]interface{})
	for k, v := range converted {
		copied[k] = v
	}

	return copied, nil
}

// configMapFilter returns filtering function for single configmap
func configMapFilter(name string) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return false
		}
		return cm.ObjectMeta.Name == name
	}
}

type configMapEventHandler struct {
	out chan map[string]string
}

func (x *configMapEventHandler) OnAdd(obj interface{}) {
	cm := obj.(*corev1.ConfigMap)
	x.out <- cm.Data
}
func (x *configMapEventHandler) OnUpdate(oldObj, newObj interface{}) {
	cm := newObj.(*corev1.ConfigMap)
	x.out <- cm.Data
}

func (x *configMapEventHandler) OnDelete(obj interface{}) {
	// noop
}
