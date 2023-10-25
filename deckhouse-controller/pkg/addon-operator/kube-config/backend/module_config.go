// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backend

import (
	"context"
	"errors"
	"time"

	logger "github.com/docker/distribution/context"
	"github.com/flant/addon-operator/pkg/kube_config_manager/config"
	"github.com/flant/addon-operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/clientset/versioned"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/informers/externalversions"
)

type ModuleConfig struct {
	mcKubeClient *versioned.Clientset
	logger       logger.Logger
}

// New returns native(Deckhouse) implementation for addon-operator's KubeConfigManager which works directly with
// deckhouse.io/ModuleConfig, avoiding moving configs to the ConfigMap
func New(config *rest.Config, logger logger.Logger) *ModuleConfig {
	mcClient, err := versioned.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return &ModuleConfig{
		mcClient,
		logger,
	}
}

func (mc ModuleConfig) StartInformer(ctx context.Context, eventC chan config.Event) {
	// define resyncPeriod for informer
	resyncPeriod := time.Duration(15) * time.Minute

	informer := externalversions.NewSharedInformerFactory(mc.mcKubeClient, resyncPeriod)
	mcInformer := informer.Deckhouse().V1alpha1().ModuleConfigs().Informer()

	_, err := mcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mconfig := obj.(*v1alpha1.ModuleConfig)
			mc.handleEvent(mconfig, eventC)
		},
		UpdateFunc: func(prevObj interface{}, obj interface{}) {
			mconfig := obj.(*v1alpha1.ModuleConfig)
			mc.handleEvent(mconfig, eventC)
		},
		DeleteFunc: func(obj interface{}) {
			mc.handleEvent(obj.(*v1alpha1.ModuleConfig), eventC)
		},
	})
	if err != nil {
		// TODO: return err
		panic(err)
	}

	go func() {
		mcInformer.Run(ctx.Done())
	}()
}

func (mc ModuleConfig) handleEvent(obj *v1alpha1.ModuleConfig, eventC chan config.Event) {
	cfg := config.NewConfig()
	values := utils.Values(obj.Spec.Settings)

	if obj.DeletionTimestamp != nil {
		// ModuleConfig was deleted
		values = utils.Values{}
	}

	switch obj.Name {
	case "global":
		cfg.Global = &config.GlobalKubeConfig{
			Values:   values,
			Checksum: values.Checksum(),
		}

	default:
		mcfg := utils.NewModuleConfig(obj.Name, values)
		mcfg.IsEnabled = obj.Spec.Enabled
		cfg.Modules[obj.Name] = &config.ModuleKubeConfig{
			ModuleConfig: *mcfg,
			Checksum:     mcfg.Checksum(),
		}
	}
	eventC <- config.Event{Key: obj.Name, Config: cfg}
}

func (mc ModuleConfig) LoadConfig(ctx context.Context) (*config.KubeConfig, error) {
	// List all ModuleConfig and get settings
	cfg := config.NewConfig()

	list, err := mc.mcKubeClient.DeckhouseV1alpha1().ModuleConfigs().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, item := range list.Items {
		values := utils.Values(item.Spec.Settings)

		if item.GetName() == "global" {
			cfg.Global = &config.GlobalKubeConfig{
				Values:   values,
				Checksum: values.Checksum(),
			}
		} else {
			mcfg := utils.NewModuleConfig(item.Name, values)
			mcfg.IsEnabled = item.Spec.Enabled
			cfg.Modules[item.Name] = &config.ModuleKubeConfig{
				ModuleConfig: *mcfg,
				Checksum:     mcfg.Checksum(),
			}
		}
	}

	return cfg, nil
}

func (mc ModuleConfig) SaveConfigValues(_ context.Context, _ string, _ utils.Values) ( /*checksum*/ string, error) {
	return "", errors.New("saving patch values in ModuleConfig is forbidden")
}
