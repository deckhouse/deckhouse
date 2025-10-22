/*
Copyright 2024 Flant JSC

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

package helpers

import (
	"sync"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/telemetry"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

// DeckhouseSettings is an openapi spec for deckhouse settings, it's not a part of DeckhouseReleaseSpec but rather
// it's a part of DeckhouseReleaseController
type DeckhouseSettings struct {
	Update struct {
		Mode                   string                            `json:"mode"`
		DisruptionApprovalMode string                            `json:"disruptionApprovalMode"`
		Windows                update.Windows                    `json:"windows"`
		NotificationConfig     releaseUpdater.NotificationConfig `json:"notification"`
	} `json:"update"`
	ReleaseChannel           string `json:"releaseChannel"`
	AllowExperimentalModules bool   `json:"allowExperimentalModules"`
}

func DefaultDeckhouseSettings() *DeckhouseSettings {
	settings := &DeckhouseSettings{
		ReleaseChannel:           "",
		AllowExperimentalModules: false,
	}
	settings.Update.Mode = "Auto"
	settings.Update.DisruptionApprovalMode = "Auto"

	return settings
}

func NewDeckhouseSettingsContainer(spec *DeckhouseSettings, metricStorage metricsstorage.Storage) *DeckhouseSettingsContainer {
	return &DeckhouseSettingsContainer{settings: spec, inited: make(chan struct{}), metricStorage: metricStorage}
}

type DeckhouseSettingsContainer struct {
	settings      *DeckhouseSettings
	lock          sync.Mutex
	inited        chan struct{}
	metricStorage metricsstorage.Storage
}

// Set update settings in container
// TODO: notify controllers and requeue all releases
func (c *DeckhouseSettingsContainer) Set(settings *DeckhouseSettings) {
	if settings == nil {
		panic("argument should be defined")
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if c.settings == nil {
		c.settings = DefaultDeckhouseSettings()
		close(c.inited)
	}

	c.settings.ReleaseChannel = settings.ReleaseChannel
	c.settings.AllowExperimentalModules = settings.AllowExperimentalModules
	c.settings.Update.Mode = settings.Update.Mode
	c.settings.Update.Windows = settings.Update.Windows
	c.settings.Update.DisruptionApprovalMode = settings.Update.DisruptionApprovalMode
	c.settings.Update.NotificationConfig = settings.Update.NotificationConfig

	allowExperimentalModules := 0.

	if c.settings.AllowExperimentalModules {
		allowExperimentalModules = 1.
	}

	c.metricStorage.GaugeSet(telemetry.WrapName(metrics.ExperimentalModulesAreAllowedMetricName), allowExperimentalModules, map[string]string{"module": "deckhouse-controller"})
}

func (c *DeckhouseSettingsContainer) Get() *DeckhouseSettings {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.settings == nil {
		c.lock.Unlock()
		<-c.inited
		c.lock.Lock()
	}

	return c.settings
}

func NewModuleUpdatePolicySpecContainer(spec *v1alpha2.ModuleUpdatePolicySpec) *ModuleUpdatePolicySpecContainer {
	return &ModuleUpdatePolicySpecContainer{spec: spec}
}

type ModuleUpdatePolicySpecContainer struct {
	spec *v1alpha2.ModuleUpdatePolicySpec
	lock sync.Mutex
}

func (c *ModuleUpdatePolicySpecContainer) Set(settings *DeckhouseSettings) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.spec.ReleaseChannel = settings.ReleaseChannel
	c.spec.Update.Mode = settings.Update.Mode
	c.spec.Update.Windows = settings.Update.Windows
}

func (c *ModuleUpdatePolicySpecContainer) Get() *v1alpha2.ModuleUpdatePolicySpec {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.spec
}
