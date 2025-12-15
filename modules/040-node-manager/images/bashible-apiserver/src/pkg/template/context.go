/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/flant/kube-client/client"
	"github.com/fsnotify/fsnotify"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"

	"bashible-apiserver/pkg/template/registry"
)

const (
	contextSecretName   = "bashible-apiserver-context"
	registrySecretName  = "deckhouse-registry"
	nodeUserCRDName     = "nodeusers"
	moduleSourceCRDName = "modulesources"

	imageDigestsFile = "/var/files/images_digests.json"
	versionMapFile   = "/var/files/version_map.yml"
)

type Context interface {
	Get(contextKey string) (map[string]interface{}, error)
	GetBootstrapContext(ng string) (map[string]interface{}, error)
	GetConfigurationChecksum(ng string) (string, bool)
}

type UpdateHandler interface {
	OnUpdate()
}

type checksumSecretUpdater interface {
	OnChecksumUpdate(ngmap map[string][]byte)
}

// BashibleContext manages bashible template context
type BashibleContext struct {
	ctx context.Context
	rw  sync.RWMutex

	registrySynced bool
	contextSynced  bool

	contextBuilder *ContextBuilder

	updateHandler UpdateHandler
	secretHandler checksumSecretUpdater

	// input values checksums
	checksums map[string]string

	// data (taken by secretKey from secret) maps `contextKey` to `contextValue`,
	// the being arbitrary data for a combination of os, nodegroup, & kubeversion
	data map[string]interface{}

	stepsStorage *StepsStorage
	emitter      changesEmitter

	nodeUsersQueue                chan queueAction
	nodeUsersConfigurationChanged chan struct{}

	moduleSourcesQueue                chan queueAction
	moduleSourcesConfigurationChanged chan struct{}

	updateLocked bool

	configurationChecksums map[string]string
}

type ContextNotFoundError struct {
	Key string
}

func (e *ContextNotFoundError) Error() string {
	return fmt.Sprintf("context %q not found", e.Key)
}

type queueAction struct {
	action    string
	newObject *unstructured.Unstructured
	oldObject *unstructured.Unstructured
}

type UserConfiguration struct {
	Name string       `json:"name" yaml:"name"`
	Spec NodeUserSpec `json:"spec" yaml:"spec"`
}

func NewContext(ctx context.Context, stepsStorage *StepsStorage, kubeClient client.Client, resyncTimeout time.Duration, secretHandler checksumSecretUpdater, updateHandler UpdateHandler, ctrlManager ctrl.Manager) *BashibleContext {
	c := BashibleContext{
		ctx:                               ctx,
		updateHandler:                     updateHandler,
		secretHandler:                     secretHandler,
		contextBuilder:                    NewContextBuilder(ctx, stepsStorage),
		checksums:                         make(map[string]string),
		stepsStorage:                      stepsStorage,
		nodeUsersQueue:                    make(chan queueAction, 100),
		nodeUsersConfigurationChanged:     make(chan struct{}, 1),
		moduleSourcesQueue:                make(chan queueAction, 100),
		moduleSourcesConfigurationChanged: make(chan struct{}, 1),
		configurationChecksums:            make(map[string]string),
	}

	c.runFilesParser()

	// Bashible context and its dynamic update
	contextSecretFactory := newBashibleInformerFactory(kubeClient, resyncTimeout, "d8-cloud-instance-manager", "app=bashible-apiserver")
	nodeUserCRDFactory := newNodeUserInformerFactory(kubeClient, resyncTimeout)
	moduleSourcesFactory := newModuleSourcesInformerFactory(kubeClient, resyncTimeout, "app!=deckhouse,heritage!=deckhouse,module!=deckhouse")

	contextSecretUpdates := c.subscribe(ctx, contextSecretFactory, contextSecretName)

	registryStateCtrl := &registry.StateController{}
	registryDataCh := registryStateCtrl.SetupWithManager(ctx, ctrlManager)

	c.subscribeOnNodeUserCRD(ctx, nodeUserCRDFactory)
	c.subscribeOnModuleSource(ctx, moduleSourcesFactory)

	go c.onSecretsUpdate(ctx, contextSecretUpdates, registryDataCh)

	return &c
}

func (c *BashibleContext) subscribe(ctx context.Context, factory informers.SharedInformerFactory, secretName string) chan map[string][]byte {
	ch := make(chan map[string][]byte)

	// Launch the informer
	informer := factory.Core().V1().Secrets().Informer()
	informer.SetWatchErrorHandler(cache.DefaultWatchErrorHandler)

	go informer.Run(ctx.Done())

	// Subscribe to updates
	informer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: secretMapFilter(secretName),
		Handler:    &secretEventHandler{ch, c},
	})

	// Wait for the first sync of the informer cache, should not take long
	for !informer.HasSynced() {
		time.Sleep(200 * time.Millisecond)
	}

	return ch
}

func (c *BashibleContext) runFilesParser() {
	c.parseimagesDigestsFile()
	c.parseVersionMapFile()

	go c.runFilesWatcher()
}

func (c *BashibleContext) parseimagesDigestsFile() {
	hasher := sha256.New()              // writer
	buf := bytes.NewBuffer(nil)         // writer
	f, err := os.Open(imageDigestsFile) // reader
	if err != nil {
		klog.Fatal(err)
	}
	defer f.Close()

	mw := io.MultiWriter(hasher, buf)

	_, err = io.Copy(mw, f)
	if err != nil {
		klog.Fatal(err)
	}

	fileHash := fmt.Sprintf("%x", hasher.Sum(nil))
	if c.isChecksumEqual(imageDigestsFile, fileHash) {
		return
	}

	var imagesDigests map[string]map[string]string

	err = json.NewDecoder(buf).Decode(&imagesDigests)
	if err != nil {
		klog.Fatalf("images_digests.json unmarshal error: %v", err)
	}

	c.contextBuilder.SetImagesData(imagesDigests)
	c.saveChecksum(imageDigestsFile, fileHash)

	klog.Info("images_digests.json file has been changed")

	c.update("file: images_tags")
}

func (c *BashibleContext) parseVersionMapFile() {
	hasher := sha256.New()            // writer
	buf := bytes.NewBuffer(nil)       // writer
	f, err := os.Open(versionMapFile) // reader
	if err != nil {
		klog.Fatal(err)
	}
	defer f.Close()

	mw := io.MultiWriter(hasher, buf)

	_, err = io.Copy(mw, f)
	if err != nil {
		klog.Fatal(err)
	}

	fileHash := fmt.Sprintf("%x", hasher.Sum(nil))
	if c.isChecksumEqual(versionMapFile, fileHash) {
		return
	}

	var versionMap map[string]interface{}

	err = yaml.Unmarshal(buf.Bytes(), &versionMap)
	if err != nil {
		klog.Fatalf("version_map.yml unmarshal error: %v", err)
	}

	klog.Info("version_map.yml file has been changed")

	c.contextBuilder.SetVersionMapData(versionMap)
	c.saveChecksum(versionMapFile, fileHash)

	c.update("file: version_map")
}

func (c *BashibleContext) runFilesWatcher() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		klog.Fatal(err)
	}
	defer watcher.Close()

	err = watcher.Add(versionMapFile)
	if err != nil {
		klog.Fatal(err)
	}
	err = watcher.Add(imageDigestsFile)
	if err != nil {
		klog.Fatal(err)
	}

	for {
		select {
		case event := <-watcher.Events:
			if event.Op == fsnotify.Remove {
				// k8s configmaps use symlinks,
				// old file is deleted and a new link with the same name is created
				_ = watcher.Remove(event.Name)
				err = watcher.Add(event.Name)
				if err != nil {
					klog.Fatal(err)
				}
				switch event.Name {
				case imageDigestsFile:
					go c.parseimagesDigestsFile()
				case versionMapFile:
					go c.parseVersionMapFile()
				}
			}

		case err := <-watcher.Errors:
			klog.Errorf("watch files error: %s", err)

		case <-c.ctx.Done():
			return
		}
	}
}

func (c *BashibleContext) onSecretsUpdate(ctx context.Context, contextSecretC chan map[string][]byte, registryDataCh <-chan registry.HashedRegistryData) {
	for {
		select {
		case data := <-contextSecretC:
			var input inputData
			dataKey := "input.yaml"
			inputBytes := data[dataKey]
			hash := sha256.New()
			checksum := fmt.Sprintf("%x", hash.Sum(inputBytes))
			if c.isChecksumEqual(dataKey, checksum) {
				continue
			}
			err := yaml.Unmarshal(inputBytes, &input)
			if err != nil {
				klog.Errorf("unmarshal input.yaml failed: %s", err)
				continue
			}
			c.contextBuilder.SetInputData(input)
			c.contextSynced = true
			c.saveChecksum(dataKey, checksum)
			c.update("secret: bashible-apiserver-context")

		case registryData := <-registryDataCh:
			if c.isChecksumEqual("registry", registryData.HashSum) {
				continue
			}
			c.contextBuilder.SetRegistryData(registryData.Data)
			c.registrySynced = true
			c.saveChecksum("registry", registryData.HashSum)
			c.update("secret: registry")

		case <-c.stepsStorage.OnNodeGroupConfigurationsChanged():
			c.update("NodeGroupConfiguration")

		case <-c.OnNodeUserConfigurationsChanged():
			c.update("NodeUserConfiguration")

		case <-c.OnModuleSourceChanged():
			c.update("ModuleSourceConfiguration")

		case <-ctx.Done():
			return
		}
	}
}

func (c *BashibleContext) update(src string) {
	c.rw.Lock()
	defer c.rw.Unlock()

	if c.updateLocked {
		klog.Infof("Context update is locked (source=%s)", src)
		return
	}

	if !c.contextSynced || !c.registrySynced {
		return
	}

	klog.Infof("Running context update. (Source: '%s')", src)

	// renderErr contains errors only from template rendering. We always have data here
	data, ngmap, checksumErrors := c.contextBuilder.Build()

	// easiest way to make appropriate map[string]interface{} struct
	rawData, err := yaml.Marshal(data.Map())
	if err != nil {
		klog.Errorf("Failed to marshal data: %v", err)
		return
	}

	// write for ability to check generated context from container
	_ = os.WriteFile("/tmp/context.yaml", rawData, 0666)

	if len(checksumErrors) > 0 {
		klog.Warning("Context was saved without checksums. Bashible context hasn't been upgraded")
		var errStr strings.Builder
		for bundle, err := range checksumErrors {
			_, _ = errStr.WriteString(fmt.Sprintf("\t%s: %s\n", bundle, err))
		}
		klog.Warningf("bundles checksums have errors:\n%s", errStr.String())
		_ = os.WriteFile("/tmp/context.error", []byte(errStr.String()), 0644)
		return
	}

	_ = os.Remove("/tmp/context.error")

	c.configurationChecksums = make(map[string]string, len(ngmap))
	for ng, sum := range ngmap {
		c.configurationChecksums[ng] = string(sum)
	}

	var res map[string]interface{}

	err = yaml.Unmarshal(rawData, &res)
	if err != nil {
		klog.Errorf("Failed to unmarshal data: %v", err)
		return
	}

	c.data = res

	c.secretHandler.OnChecksumUpdate(ngmap)
	c.updateHandler.OnUpdate()

}

// Get retrieves a copy of context for the given secretKey.
//
// TODO In future, node group name will be passed instead of a secretKey.
func (c *BashibleContext) Get(contextKey string) (map[string]interface{}, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	raw, ok := c.data[contextKey]
	if !ok {
		return nil, contextNotFoundError(contextKey)
	}

	converted, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("cannot convert context for secretKey \"%s\" to map[string]interface{}", contextKey)
	}

	copied := make(map[string]interface{})
	for k, v := range converted {
		copied[k] = v
	}

	return copied, nil
}

// Get retrieves a copy of context for the given secretKey.
func (c *BashibleContext) GetBootstrapContext(contextKey string) (map[string]interface{}, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	raw, ok := c.data[contextKey]
	if !ok {
		return nil, contextNotFoundError(contextKey)
	}

	converted, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("cannot convert context for secretKey \"%s\" to map[string]interface{}", contextKey)
	}

	copied := make(map[string]interface{})
	for k, v := range converted {
		copied[k] = v
	}

	return copied, nil
}

func contextNotFoundError(contextKey string) error {
	return &ContextNotFoundError{Key: contextKey}
}

// GetConfigurationChecksum returns previously calculated configuration checksum for nodegroup.
func (c *BashibleContext) GetConfigurationChecksum(ng string) (string, bool) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	sum, ok := c.configurationChecksums[ng]
	if !ok || sum == "" {
		return "", false
	}
	return sum, true
}

// secretMapFilter returns filtering function for single secret
func secretMapFilter(name string) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		cm, ok := obj.(*corev1.Secret)
		if !ok {
			return false
		}
		return cm.ObjectMeta.Name == name
	}
}

type secretEventHandler struct {
	out             chan map[string][]byte
	bashibleContext *BashibleContext
}

func (x *secretEventHandler) OnAdd(obj interface{}, _ bool) {
	secret := obj.(*corev1.Secret)

	if x.lockApplied(secret) {
		return
	}

	x.out <- secret.Data
}

func (x *secretEventHandler) OnUpdate(_, newObj interface{}) {
	secret := newObj.(*corev1.Secret)

	if x.lockApplied(secret) {
		return
	}

	x.out <- secret.Data
}

func (x *secretEventHandler) lockApplied(secret *corev1.Secret) bool {
	if v, ok := secret.Annotations["node.deckhouse.io/bashible-locked"]; ok {
		if v == "true" {
			x.bashibleContext.updateLocked = true
			return true
		}
	} else {
		x.bashibleContext.updateLocked = false
	}

	return false
}

func (x *secretEventHandler) OnDelete(obj interface{}) {
	// noop
}

func (c *BashibleContext) isChecksumEqual(name, newChecksum string) bool {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if oldChecksum, ok := c.checksums[name]; ok {
		return oldChecksum == newChecksum
	}

	return false
}

func (c *BashibleContext) saveChecksum(name, newChecksum string) {
	c.rw.Lock()
	defer c.rw.Unlock()

	c.checksums[name] = newChecksum
}

// newBashibleInformerFactory creates informer factory for particular namespace and label selector.
// Bashible apiserver is expected to use single namespace and only related resources.
func newBashibleInformerFactory(kubeClient client.Client, resync time.Duration, namespace, labelSelector string) informers.SharedInformerFactory {
	factory := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		resync,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = labelSelector
		}),
	)

	return factory
}

func newNodeUserInformerFactory(kubeClient client.Client, resync time.Duration) dynamicinformer.DynamicSharedInformerFactory {
	factory := dynamicinformer.NewDynamicSharedInformerFactory(
		kubeClient.Dynamic(),
		resync,
	)

	return factory
}

func newModuleSourcesInformerFactory(kubeClient client.Client, resync time.Duration, labelSelector string) dynamicinformer.DynamicSharedInformerFactory {
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		kubeClient.Dynamic(),
		resync,
		"", // ns
		dynamicinformer.TweakListOptionsFunc(func(opts *metav1.ListOptions) {
			opts.LabelSelector = labelSelector
		}),
	)

	return factory
}

func (c *BashibleContext) subscribeOnNodeUserCRD(ctx context.Context, ngConfigFactory dynamicinformer.DynamicSharedInformerFactory) {
	if ngConfigFactory == nil {
		return
	}

	go c.emitter.runBufferedEmitter(c.nodeUsersConfigurationChanged)
	go c.runNodeUserCRDQueue(ctx)

	// Launch the informer
	ginformer := ngConfigFactory.ForResource(schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1",
		Resource: nodeUserCRDName,
	})

	informer := ginformer.Informer()
	informer.SetWatchErrorHandler(cache.DefaultWatchErrorHandler)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.nodeUsersQueue <- queueAction{
				action:    "add",
				newObject: obj.(*unstructured.Unstructured),
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.nodeUsersQueue <- queueAction{
				action:    "update",
				newObject: newObj.(*unstructured.Unstructured),
				oldObject: oldObj.(*unstructured.Unstructured),
			}
		},
		DeleteFunc: func(obj interface{}) {
			c.nodeUsersQueue <- queueAction{
				action:    "delete",
				oldObject: obj.(*unstructured.Unstructured),
			}
		},
	})

	go informer.Run(ctx.Done())

	// Wait for the first sync of the informer cache, should not take long
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		klog.Fatalf("unable to sync caches: %v", ctx.Err())
	}
}

func (c *BashibleContext) subscribeOnModuleSource(ctx context.Context, moduleSourcesFactory dynamicinformer.DynamicSharedInformerFactory) {
	if moduleSourcesFactory == nil {
		return
	}

	go c.emitter.runBufferedEmitter(c.moduleSourcesConfigurationChanged)
	go c.runModuleSourceCRDQueue(ctx)

	ginformer := moduleSourcesFactory.ForResource(schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: moduleSourceCRDName,
	})

	informer := ginformer.Informer()
	informer.SetWatchErrorHandler(cache.DefaultWatchErrorHandler)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.moduleSourcesQueue <- queueAction{
				action:    "add",
				newObject: obj.(*unstructured.Unstructured),
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.moduleSourcesQueue <- queueAction{
				action:    "update",
				newObject: newObj.(*unstructured.Unstructured),
				oldObject: oldObj.(*unstructured.Unstructured),
			}
		},
		DeleteFunc: func(obj interface{}) {
			c.moduleSourcesQueue <- queueAction{
				action:    "delete",
				oldObject: obj.(*unstructured.Unstructured),
			}
		},
	})

	go informer.Run(ctx.Done())

	// Wait for the first sync of the informer cache, should not take long
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		klog.Fatalf("unable to sync caches: %v", ctx.Err())
	}
}

func (c *BashibleContext) AddNodeUserConfiguration(nu *NodeUser) {
	klog.Infof("Adding NodeUser %s to context", nu.Name)
	ngPairs := generateNgPairs(nu.Spec.NodeGroups)

	nuc := UserConfiguration{
		Name: nu.Name,
		Spec: nu.Spec,
	}

	c.rw.Lock()
	defer c.rw.Unlock()
	for _, ngPair := range ngPairs {
		if m, ok := c.contextBuilder.nodeUserConfigurations[ngPair]; ok {
			m = append(m, &nuc)
			c.contextBuilder.nodeUserConfigurations[ngPair] = m
		} else {
			c.contextBuilder.nodeUserConfigurations[ngPair] = []*UserConfiguration{&nuc}
		}
	}
}

func (c *BashibleContext) AddModuleSourceCA(ms *ModuleSource) {
	klog.Infof("Adding CA ModuleSource %s to context", ms.Name)
	c.rw.Lock()
	defer c.rw.Unlock()
	registryHost := ms.getRegistry()
	if _, ok := c.contextBuilder.moduleSourcesCA[registryHost]; !ok {
		c.contextBuilder.moduleSourcesCA[registryHost] = ms.Spec.Registry.CA
	}
}

func (c *BashibleContext) RemoveModuleSourceCA(ms *ModuleSource) {
	klog.Infof("Removing CA ModuleSource %s from context", ms.Name)

	c.rw.Lock()
	defer c.rw.Unlock()
	registryHost := ms.getRegistry()
	delete(c.contextBuilder.moduleSourcesCA, registryHost)

}

func (c *BashibleContext) RemoveNodeUserConfiguration(nu *NodeUser) {
	klog.Infof("Removing NodeUser %s from context", nu.Name)
	ngPairs := generateNgPairs(nu.Spec.NodeGroups)

	c.rw.Lock()
	defer c.rw.Unlock()
	for _, ngPair := range ngPairs {
		if configs, ok := c.contextBuilder.nodeUserConfigurations[ngPair]; ok {
			for i, v := range configs {
				if v.Name == nu.Name {
					configs = append(configs[:i], configs[i+1:]...)
					break
				}
			}
			c.contextBuilder.nodeUserConfigurations[ngPair] = configs
		}
	}
}

func (c *BashibleContext) runNodeUserCRDQueue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case event := <-c.nodeUsersQueue:
			switch event.action {
			case "add":
				var nu NodeUser
				err := fromUnstructured(event.newObject, &nu)
				if err != nil {
					klog.Errorf("Action: add, nodeUser: %s - convert from unstructured failed: %v", event.newObject.GetName(), err)
					continue
				}
				c.AddNodeUserConfiguration(&nu)

			case "update":
				var newConf NodeUser
				err := fromUnstructured(event.newObject, &newConf)
				if err != nil {
					klog.Errorf("Action: update, nodeUser: %s - convert from unstructured failed: %v", event.newObject.GetName(), err)
					continue
				}

				var oldConf NodeUser
				err = fromUnstructured(event.oldObject, &oldConf)
				if err != nil {
					klog.Errorf("Action: update, nodeUser: %s - convert from unstructured failed: %v", event.newObject.GetName(), err)
					continue
				}

				if newConf.Spec.IsEqual(oldConf.Spec) {
					continue
				}

				c.RemoveNodeUserConfiguration(&oldConf)
				c.AddNodeUserConfiguration(&newConf)

			case "delete":
				var nu NodeUser
				err := fromUnstructured(event.oldObject, &nu)
				if err != nil {
					klog.Errorf("Action: delete, nodeUser: %s - convert from unstructured failed: %v", event.newObject.GetName(), err)
					continue
				}
				c.RemoveNodeUserConfiguration(&nu)
			}

			c.emitter.emitChanges()
		}
	}
}

func (c *BashibleContext) runModuleSourceCRDQueue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case event := <-c.moduleSourcesQueue:
			switch event.action {
			case "add":
				var ms ModuleSource
				err := fromUnstructured(event.newObject, &ms)
				if err != nil {
					klog.Errorf("Action: add, moduleSource: %s - convert from unstructured failed: %v", event.newObject.GetName(), err)
					continue
				}
				c.AddModuleSourceCA(&ms)

			case "update":
				var newModuleSourceConf ModuleSource
				err := fromUnstructured(event.newObject, &newModuleSourceConf)
				if err != nil {
					klog.Errorf("Action: update, moduleSource: %s - convert from unstructured failed: %v", event.newObject.GetName(), err)
					continue
				}

				var oldModuleSourceConf ModuleSource
				err = fromUnstructured(event.oldObject, &oldModuleSourceConf)
				if err != nil {
					klog.Errorf("Action: update, moduleSource: %s - convert from unstructured failed: %v", event.newObject.GetName(), err)
					continue
				}

				if newModuleSourceConf.IsEqual(oldModuleSourceConf) {
					continue
				}

				c.RemoveModuleSourceCA(&oldModuleSourceConf)
				c.AddModuleSourceCA(&newModuleSourceConf)

			case "delete":
				var ms ModuleSource
				err := fromUnstructured(event.oldObject, &ms)
				if err != nil {
					klog.Errorf("Action: delete, moduleSource: %s - convert from unstructured failed: %v", event.newObject.GetName(), err)
					continue
				}
				c.RemoveModuleSourceCA(&ms)
			}

			c.emitter.emitChanges()
		}
	}
}

func (c *BashibleContext) OnNodeUserConfigurationsChanged() chan struct{} {
	return c.nodeUsersConfigurationChanged
}

func (c *BashibleContext) OnModuleSourceChanged() chan struct{} {
	return c.moduleSourcesConfigurationChanged
}
