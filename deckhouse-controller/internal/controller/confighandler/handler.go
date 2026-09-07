// Copyright 2026 Flant JSC
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

package confighandler

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/flant/addon-operator/pkg/kube_config_manager/backend"
	"github.com/flant/addon-operator/pkg/kube_config_manager/config"
	"github.com/flant/addon-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
)

const (
	moduleDeckhouse = "deckhouse"
	moduleGlobal    = "global"
)

var _ backend.ConfigHandler = &Handler{}

type Handler struct {
	client            client.Client
	conversionsStore  *conversion.ConversionsStore
	deckhouseConfigCh chan<- utils.Values

	l             sync.Mutex
	configEventCh chan<- config.Event
}

func New(client client.Client, conversionsStore *conversion.ConversionsStore, deckhouseConfigCh chan<- utils.Values) *Handler {
	return &Handler{
		client:            client,
		conversionsStore:  conversionsStore,
		deckhouseConfigCh: deckhouseConfigCh,
	}
}

func (h *Handler) ModuleConfigChannelIsSet() bool {
	h.l.Lock()
	defer h.l.Unlock()
	return h.configEventCh != nil
}

// HandleEvent sends event to addon-operator
func (h *Handler) HandleEvent(module *v1alpha2.Module, op config.Op) {
	kubeConfig := config.NewConfig()

	values, err := h.valuesByModuleConfig(module)
	if err != nil {
		h.configEventCh <- config.Event{Key: module.Name, Config: kubeConfig, Err: err}
		return
	}

	if module.Name == moduleGlobal {
		kubeConfig.Global = &config.GlobalKubeConfig{
			Values:   values,
			Checksum: values.Checksum(),
		}
	} else {
		addonOperatorModuleConfig := utils.NewModuleConfig(module.Name, values)
		addonOperatorModuleConfig.IsEnabled = module.Spec.Enabled
		if len(module.Spec.Maintenance) > 0 {
			addonOperatorModuleConfig.Maintenance = utils.Maintenance(module.Spec.Maintenance)
		}
		kubeConfig.Modules[module.Name] = &config.ModuleKubeConfig{
			ModuleConfig: *addonOperatorModuleConfig,
			Checksum:     addonOperatorModuleConfig.Checksum(),
		}

		// it is needed to trigger kube config apply after enabling
		if module.Spec.Enabled != nil && !*module.Spec.Enabled {
			kubeConfig.Modules[module.Name].Checksum = ""
		}

		// update deckhouse settings
		if module.Name == moduleDeckhouse {
			h.deckhouseConfigCh <- values
		}
	}

	h.configEventCh <- config.Event{Key: module.Name, Config: kubeConfig, Op: op}
}

// StartInformer does not start informer, it just registers channels, this name is used just to implement interface
func (h *Handler) StartInformer(_ context.Context, eventCh chan config.Event) {
	h.l.Lock()
	h.configEventCh = eventCh
	h.l.Unlock()
}

// LoadConfig loads initial modules config before starting
func (h *Handler) LoadConfig(ctx context.Context, _ ...string) (*config.KubeConfig, error) {
	modules := new(v1alpha2.ModuleList)
	if err := h.client.List(ctx, modules); err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	kubeConfig := config.NewConfig()
	for _, module := range modules.Items {
		values, err := h.valuesByModuleConfig(&module)
		if err != nil {
			return nil, err
		}

		if module.Name == moduleGlobal {
			kubeConfig.Global = &config.GlobalKubeConfig{
				Values:   values,
				Checksum: values.Checksum(),
			}
			continue
		}

		addonOperatorModuleConfig := utils.NewModuleConfig(module.Name, values)
		addonOperatorModuleConfig.IsEnabled = module.Spec.Enabled
		if len(module.Spec.Maintenance) > 0 {
			addonOperatorModuleConfig.Maintenance = utils.Maintenance(module.Spec.Maintenance)
		}
		kubeConfig.Modules[module.Name] = &config.ModuleKubeConfig{
			ModuleConfig: *addonOperatorModuleConfig,
			Checksum:     addonOperatorModuleConfig.Checksum(),
		}

		// update deckhouse settings
		if module.Name == moduleDeckhouse {
			h.deckhouseConfigCh <- values
		}
	}

	return kubeConfig, nil
}

func (h *Handler) valuesByModuleConfig(module *v1alpha2.Module) (utils.Values, error) {
	if module.DeletionTimestamp != nil {
		// ModuleConfig was deleted
		return utils.Values{}, nil
	}

	settings := module.Spec.Settings.GetMap()

	if module.Spec.SettingsVersion == 0 {
		return utils.Values(settings), nil
	}

	converter := h.conversionsStore.Get(module.Name)
	newVersion, newSettings, err := converter.ConvertToLatest(module.Spec.SettingsVersion, settings)
	if err != nil {
		return utils.Values{}, fmt.Errorf("convert to latest: %w", err)
	}

	module.Spec.SettingsVersion = newVersion
	module.Spec.Settings = v1alpha1.MakeMappedFields(newSettings)

	return utils.Values(newSettings), nil
}

// SaveConfigValues saving patches in ModuleConfigBackend.
// Deprecated
func (h *Handler) SaveConfigValues(_ context.Context, _ string, _ utils.Values) (string, error) {
	return "", errors.New("saving patch values in ModuleConfig is forbidden")
}
