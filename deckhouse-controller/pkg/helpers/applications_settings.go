/*
Copyright 2026 Flant JSC

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
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
)

// ApplicationsSettingsContainer holds global.applications.update, parsed once per change of the
// global settings, the way DeckhouseSettingsContainer holds the deckhouse ones.
type ApplicationsSettingsContainer struct {
	lock sync.Mutex

	// update is nil while the global settings do not set applications.update.
	update *releaseUpdater.Settings

	// subscribers are signalled when Set changes update.
	subscribers []chan struct{}
}

func NewApplicationsSettingsContainer() *ApplicationsSettingsContainer {
	return new(ApplicationsSettingsContainer)
}

// Set parses applications.update out of the global settings and signals the subscribers if it
// changed. Settings that fail to parse leave the stored ones in place.
func (c *ApplicationsSettingsContainer) Set(global addonutils.Values) error {
	parsed, err := parseApplicationsUpdate(global)
	if err != nil {
		return err
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if reflect.DeepEqual(c.update, parsed) {
		return nil
	}

	c.update = parsed

	for _, subscriber := range c.subscribers {
		select {
		case subscriber <- struct{}{}:
		default: // a signal is already pending, and it covers this change as well
		}
	}

	return nil
}

// GetUpdateSettings returns global.applications.update, and false while the global settings do
// not set it.
func (c *ApplicationsSettingsContainer) GetUpdateSettings() (releaseUpdater.Settings, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.update == nil {
		return releaseUpdater.Settings{}, false
	}

	return *c.update, true
}

// SubscribeUpdate returns a channel signalled whenever Set changes applications.update. Signals do
// not queue up: a subscriber that reads late gets one for all the changes in between, so it has to
// read the settings back rather than count the signals.
func (c *ApplicationsSettingsContainer) SubscribeUpdate() <-chan struct{} {
	c.lock.Lock()
	defer c.lock.Unlock()

	subscriber := make(chan struct{}, 1)
	c.subscribers = append(c.subscribers, subscriber)

	return subscriber
}

// parseApplicationsUpdate returns applications.update of the global settings over its defaults, or
// nil when the settings do not set it.
func parseApplicationsUpdate(global addonutils.Values) (*releaseUpdater.Settings, error) {
	// A round trip through JSON rather than a walk over the map: the nested values are typed
	// differently depending on whether they came from a ModuleConfig or from a package.
	raw, err := json.Marshal(global)
	if err != nil {
		return nil, fmt.Errorf("marshal the global settings: %w", err)
	}

	var settings struct {
		Applications struct {
			Update json.RawMessage `json:"update"`
		} `json:"applications"`
	}

	if err = json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("unmarshal the global settings: %w", err)
	}

	if len(settings.Applications.Update) == 0 || string(settings.Applications.Update) == "null" {
		return nil, nil
	}

	applications := struct {
		Mode          string                            `json:"mode"`
		Windows       update.Windows                    `json:"windows"`
		Notification  releaseUpdater.NotificationConfig `json:"notification"`
		BlockOnAlerts releaseUpdater.BlockOnAlerts      `json:"blockOnAlerts"`
	}{
		BlockOnAlerts: releaseUpdater.BlockOnAlerts{Severity: DefaultBlockOnAlertsSeverity},
	}

	if err = json.Unmarshal(settings.Applications.Update, &applications); err != nil {
		return nil, fmt.Errorf("unmarshal applications.update: %w", err)
	}

	return &releaseUpdater.Settings{
		Mode:               v1alpha2.ParseUpdateMode(applications.Mode),
		Windows:            applications.Windows,
		NotificationConfig: applications.Notification,
		BlockOnAlerts:      applications.BlockOnAlerts,
	}, nil
}
