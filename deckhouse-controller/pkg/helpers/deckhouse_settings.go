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
	"reflect"
	"slices"
	"sync"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/telemetry"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

// DefaultBlockOnAlertsSeverity is the default of update.blockOnAlerts.severity in the deckhouse
// settings, in global.applications.update and in the Application spec alike.
const DefaultBlockOnAlertsSeverity = 4

// DeckhouseSettings is an openapi spec for deckhouse settings, it's not a part of DeckhouseReleaseSpec but rather
// it's a part of DeckhouseReleaseController
type DeckhouseSettings struct {
	Update struct {
		Mode                   string                            `json:"mode"`
		DisruptionApprovalMode string                            `json:"disruptionApprovalMode"`
		Windows                update.Windows                    `json:"windows"`
		NotificationConfig     releaseUpdater.NotificationConfig `json:"notification"`
		BlockOnAlerts          releaseUpdater.BlockOnAlerts      `json:"blockOnAlerts"`
	} `json:"update"`
	ReleaseChannel             string   `json:"releaseChannel"`
	AllowExperimentalModules   bool     `json:"allowExperimentalModules"`
	AllowedExperimentalModules []string `json:"allowedExperimentalModules"`
}

func DefaultDeckhouseSettings() *DeckhouseSettings {
	settings := &DeckhouseSettings{
		ReleaseChannel:           "",
		AllowExperimentalModules: false,
	}
	// The documented OpenAPI default for settings.update.mode is AutoPatch
	// (modules/002-deckhouse/openapi/config-values.yaml). This seed is applied
	// before the deckhouse ModuleConfig is unmarshalled over it in
	// syncDeckhouseSettings, and json.Unmarshal leaves an unset update.mode
	// untouched — so this default is what an unset mode resolves to. It must
	// match the OpenAPI default (and ParseUpdateMode's fallback), otherwise a
	// cluster without an explicit update.mode silently runs in Auto and
	// auto-applies minor releases without approval.
	settings.Update.Mode = v1alpha2.UpdateModeAutoPatch.String()
	settings.Update.DisruptionApprovalMode = "Auto"
	settings.Update.BlockOnAlerts.Enabled = false
	settings.Update.BlockOnAlerts.Severity = DefaultBlockOnAlertsSeverity

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

	// updateSubscribers are signalled when Set changes the update settings.
	updateSubscribers []chan struct{}
}

// Set update settings in container
// TODO: requeue all releases on a change, as SubscribeUpdate lets the Application controller do
func (c *DeckhouseSettingsContainer) Set(settings *DeckhouseSettings) {
	if settings == nil {
		panic("argument should be defined")
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	inited := c.settings != nil
	if !inited {
		c.settings = DefaultDeckhouseSettings()
		close(c.inited)
	}

	// Every field of the update settings is copied below, so this is whether the copy changes them.
	updateChanged := !inited || !reflect.DeepEqual(c.settings.Update, settings.Update)

	c.settings.ReleaseChannel = settings.ReleaseChannel
	c.settings.AllowExperimentalModules = settings.AllowExperimentalModules
	c.settings.AllowedExperimentalModules = slices.Clone(settings.AllowedExperimentalModules)
	c.settings.Update.Mode = settings.Update.Mode
	c.settings.Update.Windows = settings.Update.Windows
	c.settings.Update.DisruptionApprovalMode = settings.Update.DisruptionApprovalMode
	c.settings.Update.NotificationConfig = settings.Update.NotificationConfig
	c.settings.Update.BlockOnAlerts.Enabled = settings.Update.BlockOnAlerts.Enabled
	c.settings.Update.BlockOnAlerts.Severity = settings.Update.BlockOnAlerts.Severity

	if updateChanged {
		for _, subscriber := range c.updateSubscribers {
			select {
			case subscriber <- struct{}{}:
			default: // a signal is already pending, and it covers this change as well
			}
		}
	}

	allowExperimentalModules := 0.

	if c.settings.AllowExperimentalModules {
		allowExperimentalModules = 1.
	}

	c.metricStorage.GaugeSet(telemetry.WrapName(metrics.ExperimentalModulesAreAllowedMetricName), allowExperimentalModules, map[string]string{metrics.LabelModule: "deckhouse-controller"})
}

// SubscribeUpdate returns a channel signalled whenever Set changes the update settings, the first
// Set included. Signals do not queue up: a subscriber that reads late gets one for all the
// changes in between, so it has to read the settings back rather than count the signals.
func (c *DeckhouseSettingsContainer) SubscribeUpdate() <-chan struct{} {
	c.lock.Lock()
	defer c.lock.Unlock()

	subscriber := make(chan struct{}, 1)
	c.updateSubscribers = append(c.updateSubscribers, subscriber)

	return subscriber
}

// GetUpdateSettings returns the update settings in the form the release updater takes them. They
// are read under the lock, which reading them through Get is not, and Set replaces the windows and
// the notification settings rather than changing them in place, so the result cannot be torn. It
// blocks until the settings are first set, the same as Get.
func (c *DeckhouseSettingsContainer) GetUpdateSettings() releaseUpdater.Settings {
	c.lock.Lock()
	if c.settings == nil {
		c.lock.Unlock()
		<-c.inited
		c.lock.Lock()
	}
	defer c.lock.Unlock()

	return releaseUpdater.Settings{
		NotificationConfig:     c.settings.Update.NotificationConfig,
		BlockOnAlerts:          c.settings.Update.BlockOnAlerts,
		DisruptionApprovalMode: c.settings.Update.DisruptionApprovalMode,
		Mode:                   v1alpha2.ParseUpdateMode(c.settings.Update.Mode),
		Windows:                c.settings.Update.Windows,
	}
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

// ExperimentalModuleAllowed reports whether the named module may be enabled
// despite being experimental: either all experimental modules are allowed, or
// the module is named in the allowlist. It reads the settings under the lock.
func (c *DeckhouseSettingsContainer) ExperimentalModuleAllowed(name string) bool {
	c.lock.Lock()
	if c.settings == nil {
		c.lock.Unlock()
		<-c.inited
		c.lock.Lock()
	}
	defer c.lock.Unlock()

	return c.settings.AllowExperimentalModules || slices.Contains(c.settings.AllowedExperimentalModules, name)
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
