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
	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/conversion"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

type ModuleConfigBackend struct {
	mcKubeClient     *versioned.Clientset
	deckhouseConfigC chan<- utils.Values
	logger           logger.Logger
}

// New returns native(Deckhouse) implementation for addon-operator's KubeConfigManager which works directly with
// deckhouse.io/ModuleConfig, avoiding moving configs to the ConfigMap
func New(config *rest.Config, deckhouseConfigC chan<- utils.Values, logger logger.Logger) *ModuleConfigBackend {
	mcClient, err := versioned.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return &ModuleConfigBackend{
		mcClient,
		deckhouseConfigC,
		logger,
	}
}

func (mc ModuleConfigBackend) handleDeckhouseConfig(moduleName string, val utils.Values) {
	if moduleName != "deckhouse" {
		return
	}

	mc.deckhouseConfigC <- val
}

func (mc ModuleConfigBackend) StartInformer(ctx context.Context, eventC chan config.Event) {
	// define resyncPeriod for informer
	resyncPeriod := time.Duration(0) * time.Minute

	informer := externalversions.NewSharedInformerFactory(mc.mcKubeClient, resyncPeriod)
	mcInformer := informer.Deckhouse().V1alpha1().ModuleConfigs().Informer()

	// we can ignore the error here because we have only 1 error case here:
	//   if mcInformer was stopped already. But we are controlling its behavior
	_, _ = mcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mconfig := obj.(*v1alpha1.ModuleConfig)
			mc.handleEvent(mconfig, eventC, config.EventAdd)
		},
		UpdateFunc: func(prevObj interface{}, obj interface{}) {
			mconfig := obj.(*v1alpha1.ModuleConfig)
			mc.handleEvent(mconfig, eventC, config.EventUpdate)
		},
		DeleteFunc: func(obj interface{}) {
			mc.handleEvent(obj.(*v1alpha1.ModuleConfig), eventC, config.EventDelete)
		},
	})

	go func() {
		mcInformer.Run(ctx.Done())
	}()
}

func (mc ModuleConfigBackend) handleEvent(obj *v1alpha1.ModuleConfig, eventC chan config.Event, op config.Op) {
	cfg := config.NewConfig()

	values, err := mc.fetchValuesFromModuleConfig(obj)
	if err != nil {
		eventC <- config.Event{Key: obj.Name, Config: cfg, Err: err}
		return
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
		mc.handleDeckhouseConfig(obj.Name, values)
	}
	eventC <- config.Event{Key: obj.Name, Config: cfg, Op: op}
}

// LoadConfig loads modules configs from ModuleConfig resources, either for all modules at once or if modulesNames list is provided, then only for listed modules
func (mc ModuleConfigBackend) LoadConfig(ctx context.Context, modulesNames ...string) (*config.KubeConfig, error) {
	// List all ModuleConfig and get settings
	cfg := config.NewConfig()
	modules := set.New(modulesNames...)

	list, err := mc.mcKubeClient.DeckhouseV1alpha1().ModuleConfigs().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, item := range list.Items {
		// check if some certain modules were inquiered
		if len(modules) > 0 && !modules.Has(item.Name) {
			continue
		}

		values, err := mc.fetchValuesFromModuleConfig(&item)
		if err != nil {
			return nil, err
		}

		if item.Name == "global" {
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
			mc.handleDeckhouseConfig(item.Name, values)
		}
	}

	return cfg, nil
}

func (mc ModuleConfigBackend) fetchValuesFromModuleConfig(item *v1alpha1.ModuleConfig) (utils.Values, error) {
	if item.DeletionTimestamp != nil {
		// ModuleConfig was deleted
		return utils.Values{}, nil
	}

	if item.Spec.Version == 0 {
		// spec version not set explicitly
		return utils.Values(item.Spec.Settings), nil
	}

	chain := conversion.Registry().Chain(item.Name)
	if chain.LatestVersion() != item.Spec.Version {
		newVersion, newSettings, err := chain.ConvertToLatest(item.Spec.Version, item.Spec.Settings)
		if err != nil {
			return utils.Values{}, err
		}
		item.Spec.Version = newVersion
		item.Spec.Settings = newSettings
	}

	return utils.Values(item.Spec.Settings), nil
}

// SaveConfigValues saving patches in ModuleConfigBackend. Used for settings-conversions
func (mc ModuleConfigBackend) SaveConfigValues(_ context.Context, moduleName string, values utils.Values) ( /*checksum*/ string, error) {
	mc.logger.Errorf("module %s tries to save values in ModuleConfig: %s", moduleName, values.DebugString())
	return "", errors.New("saving patch values in ModuleConfig is forbidden")
}
