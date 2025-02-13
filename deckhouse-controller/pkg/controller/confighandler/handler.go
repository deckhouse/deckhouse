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
	"errors"

	"github.com/flant/addon-operator/pkg/kube_config_manager/backend"
	"github.com/flant/addon-operator/pkg/kube_config_manager/config"
	"github.com/flant/addon-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	moduleDeckhouse = "deckhouse"
	moduleGlobal    = "global"
)

var _ backend.ConfigHandler = &Handler{}

type Handler struct {
	client            client.Client
	log               *log.Logger
	deckhouseConfigCh chan<- utils.Values
	configEventCh     chan<- config.Event
}

func New(client client.Client, deckhouseConfigCh chan<- utils.Values, logger *log.Logger) *Handler {
	return &Handler{
		log:               logger,
		client:            client,
		deckhouseConfigCh: deckhouseConfigCh,
	}
}

func (h *Handler) ModuleConfigChannelIsSet() bool {
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
		kubeConfig.Modules[moduleConfig.Name] = &config.ModuleKubeConfig{
			ModuleConfig: *addonOperatorModuleConfig,
			Checksum:     addonOperatorModuleConfig.Checksum(),
		}

		// update deckhouse settings
		if moduleConfig.Name == moduleDeckhouse {
			h.deckhouseConfigCh <- values
		}
	}

	h.configEventCh <- config.Event{Key: moduleConfig.Name, Config: kubeConfig, Op: op}
}

// StartInformer does not start informer, it just registers channels, this name used just to implement interface
func (h *Handler) StartInformer(_ context.Context, eventCh chan config.Event) {
	h.configEventCh = eventCh
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

		kubeConfig.Modules[moduleConfig.Name] = &config.ModuleKubeConfig{
			ModuleConfig: *addonOperatorModuleConfig,
			Checksum:     addonOperatorModuleConfig.Checksum(),
		}

		// update deckhouse settings
		if moduleConfig.Name == moduleDeckhouse {
			h.deckhouseConfigCh <- values
		}
	}

	h.log.Debug("ConfigHandler loaded initial config")

	return kubeConfig, nil
}

func (h *Handler) valuesByModuleConfig(moduleConfig *v1alpha1.ModuleConfig) (utils.Values, error) {
	if moduleConfig.DeletionTimestamp != nil {
		// ModuleConfig was deleted
		return utils.Values{}, nil
	}

	if moduleConfig.Spec.Version == 0 {
		return utils.Values(moduleConfig.Spec.Settings), nil
	}

	converter := conversion.Store().Get(moduleConfig.Name)
	newVersion, newSettings, err := converter.ConvertToLatest(moduleConfig.Spec.Version, moduleConfig.Spec.Settings)
	if err != nil {
		return utils.Values{}, err
	}

	moduleConfig.Spec.Version = newVersion
	moduleConfig.Spec.Settings = newSettings

	return utils.Values(moduleConfig.Spec.Settings), nil
}

// SaveConfigValues saving patches in ModuleConfigBackend.
// Deprecated
func (h *Handler) SaveConfigValues(_ context.Context, _ string, _ utils.Values) (string, error) {
	return "", errors.New("saving patch values in ModuleConfig is forbidden")
}
