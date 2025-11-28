// Copyright 2024 Flant JSC
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
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/flant/addon-operator/pkg/kube_config_manager/backend"
	"github.com/flant/addon-operator/pkg/kube_config_manager/config"
	"github.com/flant/addon-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
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
func (h *Handler) HandleEvent(moduleConfig *v1alpha1.ModuleConfig, op config.Op) {
	kubeConfig := config.NewConfig()

	values, err := h.valuesByModuleConfig(moduleConfig)
	if err != nil {
		h.configEventCh <- config.Event{Key: moduleConfig.Name, Config: kubeConfig, Err: err}
		return
	}

	if moduleConfig.Name == moduleGlobal {
		kubeConfig.Global = &config.GlobalKubeConfig{
			Values:   values,
			Checksum: values.Checksum(),
		}
	} else {
		addonOperatorModuleConfig := utils.NewModuleConfig(moduleConfig.Name, values)
		addonOperatorModuleConfig.IsEnabled = moduleConfig.Spec.Enabled
		if len(moduleConfig.Spec.Maintenance) > 0 {
			addonOperatorModuleConfig.Maintenance = utils.Maintenance(moduleConfig.Spec.Maintenance)
		}
		kubeConfig.Modules[moduleConfig.Name] = &config.ModuleKubeConfig{
			ModuleConfig: *addonOperatorModuleConfig,
			Checksum:     addonOperatorModuleConfig.Checksum(),
		}

		// it is needed to trigger kube config apply after enabling
		if moduleConfig.Spec.Enabled != nil && !*moduleConfig.Spec.Enabled {
			kubeConfig.Modules[moduleConfig.Name].Checksum = ""
		}

		// update deckhouse settings
		if moduleConfig.Name == moduleDeckhouse {
			h.deckhouseConfigCh <- values
		}
	}

	h.configEventCh <- config.Event{Key: moduleConfig.Name, Config: kubeConfig, Op: op}
}

// StartInformer does not start informer, it just registers channels, this name is used just to implement interface
func (h *Handler) StartInformer(_ context.Context, eventCh chan config.Event) {
	h.l.Lock()
	h.configEventCh = eventCh
	h.l.Unlock()
}

// LoadConfig loads initial modules config before starting
func (h *Handler) LoadConfig(ctx context.Context, _ ...string) (*config.KubeConfig, error) {
	configs := new(v1alpha1.ModuleConfigList)
	if err := h.client.List(ctx, configs); err != nil {
		return nil, err
	}

	kubeConfig := config.NewConfig()
	for _, moduleConfig := range configs.Items {
		values, err := h.valuesByModuleConfig(&moduleConfig)
		if err != nil {
			return nil, err
		}

		if moduleConfig.Name == moduleGlobal {
			kubeConfig.Global = &config.GlobalKubeConfig{
				Values:   values,
				Checksum: values.Checksum(),
			}
			continue
		}

		addonOperatorModuleConfig := utils.NewModuleConfig(moduleConfig.Name, values)
		addonOperatorModuleConfig.IsEnabled = moduleConfig.Spec.Enabled
		if len(moduleConfig.Spec.Maintenance) > 0 {
			addonOperatorModuleConfig.Maintenance = utils.Maintenance(moduleConfig.Spec.Maintenance)
		}
		kubeConfig.Modules[moduleConfig.Name] = &config.ModuleKubeConfig{
			ModuleConfig: *addonOperatorModuleConfig,
			Checksum:     addonOperatorModuleConfig.Checksum(),
		}

		// update deckhouse settings
		if moduleConfig.Name == moduleDeckhouse {
			h.deckhouseConfigCh <- values
		}
	}

	return kubeConfig, nil
}

func (h *Handler) valuesByModuleConfig(moduleConfig *v1alpha1.ModuleConfig) (utils.Values, error) {
	if moduleConfig.DeletionTimestamp != nil {
		// ModuleConfig was deleted
		return utils.Values{}, nil
	}

	var settings map[string]any
	if moduleConfig.Spec.Settings != nil && len(moduleConfig.Spec.Settings.Raw) > 0 {
		err := json.Unmarshal(moduleConfig.Spec.Settings.Raw, &settings)
		if err != nil {
			return utils.Values{}, fmt.Errorf("cannot unmarshal settings of ModuleConfig %q: %w", moduleConfig.Name, err)
		}
	}

	if moduleConfig.Spec.Version == 0 {
		return utils.Values(settings), nil
	}

	converter := h.conversionsStore.Get(moduleConfig.Name)
	newVersion, newSettings, err := converter.ConvertToLatest(moduleConfig.Spec.Version, settings)
	if err != nil {
		return utils.Values{}, err
	}

	rawSettings, err := json.Marshal(newSettings)
	if err != nil {
		return utils.Values{}, fmt.Errorf("cannot marshal settings of ModuleConfig %q: %w", moduleConfig.Name, err)
	}

	moduleConfig.Spec.Version = newVersion
	moduleConfig.Spec.Settings = &v1alpha1.SettingsValues{Raw: rawSettings}

	return utils.Values(newSettings), nil
}

// SaveConfigValues saving patches in ModuleConfigBackend.
// Deprecated
func (h *Handler) SaveConfigValues(_ context.Context, _ string, _ utils.Values) (string, error) {
	return "", errors.New("saving patch values in ModuleConfig is forbidden")
}
