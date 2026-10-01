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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/libapi"
)

func globalValues(t *testing.T, settings string) addonutils.Values {
	t.Helper()

	values := make(addonutils.Values)
	require.NoError(t, json.Unmarshal([]byte(settings), &values))

	return values
}

func TestParseApplicationsUpdate(t *testing.T) {
	defaults := releaseUpdater.Settings{
		Mode:          v1alpha2.UpdateModeAutoPatch,
		BlockOnAlerts: releaseUpdater.BlockOnAlerts{Severity: DefaultBlockOnAlertsSeverity},
	}

	tests := []struct {
		name     string
		settings string
		want     *releaseUpdater.Settings
	}{
		{name: "no settings", settings: `{}`},
		{name: "no applications section", settings: `{"modules":{"publicDomainTemplate":"%s.example.com"}}`},
		{name: "applications section without update", settings: `{"applications":{"ingressClass":"nginx"}}`},
		{name: "null update", settings: `{"applications":{"update":null}}`},
		{name: "empty update is set", settings: `{"applications":{"update":{}}}`, want: &defaults},
		{
			name: "every parameter set",
			settings: `{"applications":{"update":{
				"mode":"Manual",
				"windows":[{"from":"8:00","to":"15:00","days":["Tue"]}],
				"notification":{"webhook":"https://example.com/webhook","minimalNotificationTime":"6h","releaseType":"All"},
				"blockOnAlerts":{"enabled":true,"severity":2}
			}}}`,
			want: &releaseUpdater.Settings{
				Mode:    v1alpha2.UpdateModeManual,
				Windows: update.Windows{{From: "8:00", To: "15:00", Days: []string{"Tue"}}},
				NotificationConfig: releaseUpdater.NotificationConfig{
					WebhookURL:              "https://example.com/webhook",
					MinimalNotificationTime: libapi.Duration{Duration: 6 * time.Hour},
					ReleaseType:             releaseUpdater.ReleaseTypeAll,
				},
				BlockOnAlerts: releaseUpdater.BlockOnAlerts{Enabled: true, Severity: 2},
			},
		},
		{
			name:     "severity keeps its default",
			settings: `{"applications":{"update":{"blockOnAlerts":{"enabled":true}}}}`,
			want: &releaseUpdater.Settings{
				Mode:          v1alpha2.UpdateModeAutoPatch,
				BlockOnAlerts: releaseUpdater.BlockOnAlerts{Enabled: true, Severity: DefaultBlockOnAlertsSeverity},
			},
		},
		{
			name:     "zero severity overrides the default",
			settings: `{"applications":{"update":{"blockOnAlerts":{"severity":0}}}}`,
			want: &releaseUpdater.Settings{
				Mode:          v1alpha2.UpdateModeAutoPatch,
				BlockOnAlerts: releaseUpdater.BlockOnAlerts{Severity: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseApplicationsUpdate(globalValues(t, tt.settings))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("nil settings are not set", func(t *testing.T) {
		got, err := parseApplicationsUpdate(nil)
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

func TestApplicationsSettingsContainer(t *testing.T) {
	container := NewApplicationsSettingsContainer()
	changes := container.SubscribeUpdate()

	_, ok := container.GetUpdateSettings()
	assert.False(t, ok, "nothing is set before the global settings arrive")

	require.NoError(t, container.Set(globalValues(t, `{"applications":{"ingressClass":"nginx"}}`)))
	assert.Empty(t, changes, "global settings without applications.update change nothing")

	require.NoError(t, container.Set(globalValues(t, `{"applications":{"update":{"mode":"Manual"}}}`)))
	require.Len(t, changes, 1, "setting applications.update must be signalled")
	<-changes

	settings, ok := container.GetUpdateSettings()
	require.True(t, ok)
	assert.Equal(t, v1alpha2.UpdateModeManual, settings.Mode)

	require.NoError(t, container.Set(globalValues(t, `{"applications":{"update":{"mode":"Manual"},"ingressClass":"other"}}`)))
	assert.Empty(t, changes, "a change elsewhere in the global settings must not be signalled")

	require.Error(t, container.Set(globalValues(t, `{"applications":{"update":{"mode":1}}}`)))
	settings, ok = container.GetUpdateSettings()
	require.True(t, ok, "settings that fail to parse must leave the stored ones in place")
	assert.Equal(t, v1alpha2.UpdateModeManual, settings.Mode)

	require.NoError(t, container.Set(globalValues(t, `{}`)))
	require.Len(t, changes, 1, "dropping applications.update must be signalled")

	_, ok = container.GetUpdateSettings()
	assert.False(t, ok)
}
