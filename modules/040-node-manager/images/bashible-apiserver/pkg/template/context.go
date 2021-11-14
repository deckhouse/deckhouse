/*
Copyright 2021 Flant JSC

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
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

const (
	contextSecretName  = "bashible-apiserver-context"
	registrySecretName = "deckhouse-registry"

	imageTagsFile  = "/var/files/images_tags.json"
	versionMapFile = "/var/files/version_map.yml"
)

func NewContext(ctx context.Context, contextSecretFactory, registrySecretFactory informers.SharedInformerFactory, secretHandler checksumSecretUpdater, updateHandler UpdateHandler) *BashibleContext {
	c := BashibleContext{
		ctx:            ctx,
		updateHandler:  updateHandler,
		secretHandler:  secretHandler,
		contextBuilder: NewContextBuilder(ctx, "/bashible/templates"),
		checksums:      make(map[string]string),
	}

	c.runFilesParser()

	contextSecretUpdates := c.subscribe(ctx, contextSecretFactory, contextSecretName)

	registrySecretUpdates := c.subscribe(ctx, registrySecretFactory, registrySecretName)

	go c.onSecretsUpdate(ctx, contextSecretUpdates, registrySecretUpdates)

	return &c
}

type Context interface {
	Get(contextKey string) (map[string]interface{}, error)
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

	checksums map[string]string

	// data (taken by secretKey from secret) maps `contextKey` to `contextValue`,
	// the being arbitrary data for a combination of os, nodegroup, & kubeversion
	data map[string]interface{}
}

func (c *BashibleContext) subscribe(ctx context.Context, factory informers.SharedInformerFactory, secretName string) chan map[string][]byte {
	ch := make(chan map[string][]byte)
	stopInformer := make(chan struct{})

	// Launch the informer
	informer := factory.Core().V1().Secrets().Informer()
	go informer.Run(stopInformer)

	// Subscribe to updates
	informer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: secretMapFilter(secretName),
		Handler:    &secretEventHandler{ch},
	})

	// Wait for the first sync of the informer cache, should not take long
	for !informer.HasSynced() {
		time.Sleep(200 * time.Millisecond)
	}
	go func() {
		<-ctx.Done()
		close(stopInformer)
	}()

	return ch
}

func (c *BashibleContext) runFilesParser() {
	c.parseImagesTagsFile()
	c.parseVersionMapFile()

	go c.runFilesWatcher()
}

func (c *BashibleContext) parseImagesTagsFile() {
	hasher := sha256.New()           // writer
	buf := bytes.NewBuffer(nil)      // writer
	f, err := os.Open(imageTagsFile) // reader
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
	if c.isChecksumEqual(imageTagsFile, fileHash) {
		return
	}

	var imagesTags map[string]map[string]string

	err = json.NewDecoder(buf).Decode(&imagesTags)
	if err != nil {
		klog.Fatalf("images_tags.json unmarshal error: %v", err)
	}

	c.contextBuilder.SetImagesData(imagesTags)
	c.saveChecksum(imageTagsFile, fileHash)

	klog.Info("images_tags.json file has been changed")

	c.update()
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

	c.update()
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
	err = watcher.Add(imageTagsFile)
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
				case imageTagsFile:
					go c.parseImagesTagsFile()
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

func (c *BashibleContext) onSecretsUpdate(ctx context.Context, contextSecretC, registrySecretC chan map[string][]byte) {
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
			c.update()

		case data := <-registrySecretC:
			var input registryInputData
			hash := sha256.New()
			arr := make([]string, 0, len(data))
			for k, v := range data {
				arr = append(arr, k+"_"+string(v))
			}
			sort.Strings(arr)
			for _, v := range arr {
				hash.Write([]byte(v))
			}
			checksum := fmt.Sprintf("%x", hash.Sum(nil))
			if c.isChecksumEqual("registry", checksum) {
				continue
			}
			input.FromMap(data)
			c.contextBuilder.SetRegistryData(input.toRegistry())
			c.registrySynced = true
			c.saveChecksum("registry", checksum)
			c.update()

		case <-ctx.Done():
			return
		}
	}
}

func (c *BashibleContext) update() {
	c.rw.Lock()
	defer c.rw.Unlock()

	if !c.contextSynced || !c.registrySynced {
		return
	}

	klog.Info("Running context update")

	// renderErr contains errors only from template rendering. We always have data here
	data, ngmap, checksumErrors := c.contextBuilder.Build()

	// easiest way to make appropriate map[string]interface{} struct
	rawData, err := yaml.Marshal(data.Map())
	if err != nil {
		klog.Errorf("Failed to marshal data", err)
		return
	}

	// write for ability to check generated context from container
	_ = ioutil.WriteFile("/tmp/context.yaml", rawData, 0666)

	if len(checksumErrors) > 0 {
		klog.Warning("Context was saved without checksums. Bashible context hasn't been upgraded")
		var errStr strings.Builder
		for bundle, err := range checksumErrors {
			_, _ = errStr.WriteString(fmt.Sprintf("\t%s: %s\n", bundle, err))
		}
		klog.Warningf("bundles checksums have errors:\n%s", errStr.String())
		return
	}

	var res map[string]interface{}

	err = yaml.Unmarshal(rawData, &res)
	if err != nil {
		klog.Errorf("Failed to unmarshal data", err)
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
		return nil, fmt.Errorf("context not found for secretKey \"%s\"", contextKey)
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
	out chan map[string][]byte
}

func (x *secretEventHandler) OnAdd(obj interface{}) {
	secret := obj.(*corev1.Secret)
	x.out <- secret.Data
}

func (x *secretEventHandler) OnUpdate(oldObj, newObj interface{}) {
	secret := newObj.(*corev1.Secret)
	x.out <- secret.Data
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
