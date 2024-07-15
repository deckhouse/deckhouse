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

	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/updater"
)

// DeckhouseSettings is an openapi spec for deckhouse settings, it's not a part of DeckhouseReleaseSpec but rather
// it's a part of DeckhouseReleaseController
type DeckhouseSettings struct {
	Update struct {
		Mode                   string                     `json:"mode"`
		DisruptionApprovalMode string                     `json:"disruptionApprovalMode"`
		Windows                update.Windows             `json:"windows"`
		NotificationConfig     updater.NotificationConfig `json:"notification"`
	} `json:"update"`
	ReleaseChannel string `json:"releaseChannel"`
}

func NewDeckhouseSettingsContainer(spec *DeckhouseSettings) *DeckhouseSettingsContainer {
	return &DeckhouseSettingsContainer{spec: spec}
}

type DeckhouseSettingsContainer struct {
	spec *DeckhouseSettings
	lock sync.Mutex
}

func (c *DeckhouseSettingsContainer) Set(settings *DeckhouseSettings) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.spec.ReleaseChannel = settings.ReleaseChannel
	c.spec.Update.Mode = settings.Update.Mode
	c.spec.Update.Windows = settings.Update.Windows
	c.spec.Update.DisruptionApprovalMode = settings.Update.DisruptionApprovalMode
	c.spec.Update.NotificationConfig = settings.Update.NotificationConfig
}

func (c *DeckhouseSettingsContainer) Get() *DeckhouseSettings {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.spec
}
