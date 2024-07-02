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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// NameMapper maps the name of resource to the secretKey of a template
type NameMapper func(name string) (string, error)

// StepsStorage is the storage if bashible steps for a particular steps target
type StepsStorage struct {
	rootDir string

	m sync.RWMutex
	// cache all system scripts with lazy load
	systemScripts map[string]map[string][]byte

	nodeGroupConfigurations      map[string][]*nodeConfigurationScript
	nodeGroupConfigurationsQueue chan nodeConfigurationQueueAction

	configurationsChanged chan struct{}
	emitter               changesEmitter
}

type nodeConfigurationQueueAction struct {
	action    string
	newObject *unstructured.Unstructured
	oldObject *unstructured.Unstructured
}

type nodeConfigurationScript struct {
	Name    string
	Content string
}

// NewStepsStorage creates StepsStorage for target and cloud provider.
func NewStepsStorage(ctx context.Context, rootDir string, ngConfigFactory dynamicinformer.DynamicSharedInformerFactory) *StepsStorage {
	ss := &StepsStorage{
		rootDir:                      rootDir,
		systemScripts:                make(map[string]map[string][]byte),
		nodeGroupConfigurations:      make(map[string][]*nodeConfigurationScript),
		nodeGroupConfigurationsQueue: make(chan nodeConfigurationQueueAction, 100),
		configurationsChanged:        make(chan struct{}, 1),
	}

	ss.subscribeOnCRD(ctx, ngConfigFactory)
	return ss
}

func (s *StepsStorage) Render(target, provider string, templateContext map[string]interface{}, ng ...string) (map[string]string, error) {
	steps, err := s.renderSystemScripts(target, provider, templateContext)
	if err != nil {
		return nil, err
	}

	if len(ng) > 0 {
		userConfigurations, err := s.renderNodeGroupConfigurations(ng[0], templateContext)
		if err != nil {
			klog.Errorf("Render user NodeGroupConfigurations failed: %s", err)
			return steps, nil
		}

		for k, v := range userConfigurations {
			if _, ok := steps[k]; ok {
				klog.Errorf("NodeGroupConfigurations conflicts with system script: %s", k)
				continue
			}
			steps[k] = v
		}
	}

	return steps, nil
}

func (s *StepsStorage) OnNodeGroupConfigurationsChanged() chan struct{} {
	return s.configurationsChanged
}

func (s *StepsStorage) subscribeOnCRD(ctx context.Context, ngConfigFactory dynamicinformer.DynamicSharedInformerFactory) {
	if ngConfigFactory == nil {
		return
	}

	go s.emitter.runBufferedEmitter(s.configurationsChanged)
	go s.runNodeConfigurationQueue(ctx)

	// Launch the informer
	ginformer := ngConfigFactory.ForResource(schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: "nodegroupconfigurations",
	})

	informer := ginformer.Informer()
	informer.SetWatchErrorHandler(cache.DefaultWatchErrorHandler)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			s.nodeGroupConfigurationsQueue <- nodeConfigurationQueueAction{
				action:    "add",
				newObject: obj.(*unstructured.Unstructured),
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			s.nodeGroupConfigurationsQueue <- nodeConfigurationQueueAction{
				action:    "update",
				newObject: newObj.(*unstructured.Unstructured),
				oldObject: oldObj.(*unstructured.Unstructured),
			}
		},
		DeleteFunc: func(obj interface{}) {
			s.nodeGroupConfigurationsQueue <- nodeConfigurationQueueAction{
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

func (s *StepsStorage) renderSystemScripts(target, provider string, templateContext map[string]interface{}) (map[string]string, error) {
	key := fmt.Sprintf(keyPattern, target, provider)

	s.m.RLock()
	templates, ok := s.systemScripts[key]
	s.m.RUnlock()
	if !ok {
		var err error
		templates, err = s.loadTemplates(target, provider)
		if err != nil {
			return nil, err
		}
	}

	steps := make(map[string]string)
	for name, content := range templates {
		step, err := RenderTemplate(name, content, templateContext)
		if err != nil {
			return nil, fmt.Errorf("cannot render template %q: %v", name, err)
		}
		steps[step.FileName] = step.Content.String()
	}

	return steps, nil
}

func (s *StepsStorage) loadTemplates(target, provider string) (map[string][]byte, error) {
	templates := make(map[string][]byte)
	dirs := s.lookupDirs(target, provider)
	for _, dir := range dirs {
		err := s.readTemplates(dir, templates)
		if err != nil {
			return nil, err
		}
	}

	key := fmt.Sprintf(keyPattern, target, provider)
	s.m.Lock()
	s.systemScripts[key] = templates
	s.m.Unlock()

	return templates, nil
}

// $target:$provider
var keyPattern = "%s:%s"

// Expected fs hierarchy so far
//
//	bashible/{bundle}/{target}
//	bashible/common-steps/{target}
//	cloud-providers/{provider}/bashible/{bundle}/{target}
//	cloud-providers/{provider}/bashible/common-steps/{target}
//
// Where
//
//	target   = "all" | "node-group"
//	provider = "" | "aws" | "gcp" | "openstack" | ...
func (s *StepsStorage) lookupDirs(target, provider string) []string {
	dirs := []string{
		filepath.Join(s.rootDir, "bashible", "common-steps", target),
	}

	// Are we in the cloud?
	if provider != "" {
		dirs = append(dirs,
			filepath.Join(s.rootDir, "cloud-providers", provider, "bashible", "common-steps", target),
		)
	}

	return dirs
}

func (s *StepsStorage) readTemplates(baseDir string, templates map[string][]byte) error {
	return filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if os.IsNotExist(err) {
			return filepath.SkipDir
		}

		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(info.Name(), ".sh.tpl") {
			// not template
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		templates[info.Name()] = content
		return nil
	})
}

func (s *StepsStorage) AddNodeGroupConfiguration(nc *NodeGroupConfiguration) {
	name := nc.GenerateScriptName()
	klog.Infof("Adding NodeGroupConfiguration %s to context", name)

	sc := nodeConfigurationScript{
		Name:    name,
		Content: nc.Spec.Content,
	}

	s.m.Lock()
	defer s.m.Unlock()
	for _, ngName := range nc.Spec.NodeGroups {
		if m, ok := s.nodeGroupConfigurations[ngName]; ok {
			m = append(m, &sc)
			s.nodeGroupConfigurations[ngName] = m
		} else {
			s.nodeGroupConfigurations[ngName] = []*nodeConfigurationScript{&sc}
		}
	}
}

func (s *StepsStorage) RemoveNodeGroupConfiguration(nc *NodeGroupConfiguration) {
	name := nc.GenerateScriptName()
	klog.Infof("Removing NodeGroupConfiguration %s from context", name)

	s.m.Lock()
	defer s.m.Unlock()
	for _, ngName := range nc.Spec.NodeGroups {
		if configs, ok := s.nodeGroupConfigurations[ngName]; ok {
			for i, v := range configs {
				if v.Name == name {
					configs = append(configs[:i], configs[i+1:]...)
					break
				}
			}
			s.nodeGroupConfigurations[ngName] = configs
		}
	}
}

func (s *StepsStorage) renderNodeGroupConfigurations(ng string, templateContext map[string]interface{}) (map[string]string, error) {
	configurations := make([]*nodeConfigurationScript, 0)

	key := fmt.Sprintf("%s", ng)

	s.m.RLock()
	configurations = append(configurations, s.nodeGroupConfigurations[key]...)
	s.m.RUnlock()

	steps := make(map[string]string, len(configurations))
	for _, sc := range configurations {
		step, err := RenderTemplate(sc.Name, []byte(sc.Content), templateContext)
		if err != nil {
			return nil, fmt.Errorf("cannot render node configuration %q: %v", sc.Name, err)
		}
		steps[step.FileName] = step.Content.String()
	}

	return steps, nil
}

func (s *StepsStorage) runNodeConfigurationQueue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case event := <-s.nodeGroupConfigurationsQueue:
			switch event.action {
			case "add":
				var ngc NodeGroupConfiguration
				err := fromUnstructured(event.newObject, &ngc)
				if err != nil {
					klog.Errorf("Convert from unstructured failed: %s", err)
					continue
				}
				s.AddNodeGroupConfiguration(&ngc)

			case "update":
				var newConf NodeGroupConfiguration
				err := fromUnstructured(event.newObject, &newConf)
				if err != nil {
					klog.Errorf("Convert from unstructured failed: %s", err)
					continue
				}

				var oldConf NodeGroupConfiguration
				err = fromUnstructured(event.oldObject, &oldConf)
				if err != nil {
					klog.Errorf("Convert from unstructured failed: %s", err)
					continue
				}

				if newConf.Spec.IsEqual(oldConf.Spec) {
					continue
				}

				s.RemoveNodeGroupConfiguration(&oldConf)
				s.AddNodeGroupConfiguration(&newConf)

			case "delete":
				var ngc NodeGroupConfiguration
				err := fromUnstructured(event.oldObject, &ngc)
				if err != nil {
					klog.Errorf("Convert from unstructured failed: %s", err)
					continue
				}
				s.RemoveNodeGroupConfiguration(&ngc)
			}

			s.emitter.emitChanges()
		}
	}
}
