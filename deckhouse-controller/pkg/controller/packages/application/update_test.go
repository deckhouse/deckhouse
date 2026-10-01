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

package application

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/project"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

func TestMergeUpdateSettings(t *testing.T) {
	global := releaseupdater.Settings{
		Mode:               v1alpha2.UpdateModeManual,
		Windows:            update.Windows{{From: "1:00", To: "2:00"}},
		NotificationConfig: releaseupdater.NotificationConfig{WebhookURL: "https://example.com/webhook"},
		BlockOnAlerts:      releaseupdater.BlockOnAlerts{Enabled: true, Severity: 1},
	}

	t.Run("without a policy the global settings are taken as a whole", func(t *testing.T) {
		assert.Equal(t, global, mergeUpdateSettings(nil, global))
	})

	t.Run("the policy replaces everything but the notification settings", func(t *testing.T) {
		policy := &v1alpha1.ApplicationUpdate{
			Mode:          "Auto",
			Windows:       []v1alpha1.ApplicationUpdateWindow{{From: "8:00", To: "15:00", Days: []string{"Mon"}}},
			BlockOnAlerts: &v1alpha1.ApplicationUpdateBlockOnAlerts{Enabled: true, Severity: ptr.To[int32](0)},
		}

		got := mergeUpdateSettings(policy, global)

		assert.Equal(t, releaseupdater.Settings{
			Mode:               v1alpha2.UpdateModeAuto,
			Windows:            update.Windows{{From: "8:00", To: "15:00", Days: []string{"Mon"}}},
			NotificationConfig: global.NotificationConfig,
			BlockOnAlerts:      releaseupdater.BlockOnAlerts{Enabled: true, Severity: 0},
		}, got)

		policy.Windows[0].Days[0] = "Tue"
		assert.Equal(t, []string{"Mon"}, got.Windows[0].Days, "the merged windows must not alias the application spec")
	})

	t.Run("an empty policy takes the defaults, not the global settings", func(t *testing.T) {
		assert.Equal(t, releaseupdater.Settings{
			Mode:               v1alpha2.UpdateModeAutoPatch,
			NotificationConfig: global.NotificationConfig,
			BlockOnAlerts:      releaseupdater.BlockOnAlerts{Severity: helpers.DefaultBlockOnAlertsSeverity},
		}, mergeUpdateSettings(new(v1alpha1.ApplicationUpdate), global))
	})
}

func TestGlobalUpdateSettings(t *testing.T) {
	deckhouse := helpers.DefaultDeckhouseSettings()
	deckhouse.Update.Mode = v1alpha2.UpdateModeManual.String()
	deckhouse.Update.NotificationConfig.WebhookURL = "https://deckhouse.example.com/webhook"

	t.Run("global applications settings win over the deckhouse ones", func(t *testing.T) {
		applications := helpers.NewApplicationsSettingsContainer()
		require.NoError(t, applications.Set(addonutils.Values{
			"applications": map[string]any{"update": map[string]any{"mode": "Auto"}},
		}))

		r := &reconciler{
			deckhouseSettings:    helpers.NewDeckhouseSettingsContainer(deckhouse, nil),
			applicationsSettings: applications,
		}

		assert.Equal(t, releaseupdater.Settings{
			Mode:          v1alpha2.UpdateModeAuto,
			BlockOnAlerts: releaseupdater.BlockOnAlerts{Severity: helpers.DefaultBlockOnAlertsSeverity},
		}, r.globalUpdateSettings(), "the global applications settings must be taken as a whole, notifications included")
	})

	t.Run("without global applications settings the deckhouse ones are taken", func(t *testing.T) {
		r := &reconciler{
			deckhouseSettings:    helpers.NewDeckhouseSettingsContainer(deckhouse, nil),
			applicationsSettings: helpers.NewApplicationsSettingsContainer(),
		}

		assert.Equal(t, releaseupdater.Settings{
			Mode:               v1alpha2.UpdateModeManual,
			NotificationConfig: releaseupdater.NotificationConfig{WebhookURL: "https://deckhouse.example.com/webhook"},
			BlockOnAlerts:      releaseupdater.BlockOnAlerts{Severity: helpers.DefaultBlockOnAlertsSeverity},
		}, r.globalUpdateSettings(), "the deckhouse disruption approval mode must not be passed on")
	})
}

func TestRequeueOnSettingsChange(t *testing.T) {
	scheme, err := project.Scheme()
	require.NoError(t, err)

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "first", Namespace: "foo"}},
		&v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "second", Namespace: "bar"}},
	).Build()

	metricStorage := metricstorage.NewMetricStorage(metricstorage.WithNewRegistry(), metricstorage.WithLogger(log.NewNop()))
	deckhouse := helpers.NewDeckhouseSettingsContainer(helpers.DefaultDeckhouseSettings(), metricStorage)
	applications := helpers.NewApplicationsSettingsContainer()

	r := &reconciler{client: cl, logger: log.NewNop()}
	src := r.requeueOnSettingsChange(applications.SubscribeUpdate(), deckhouse.SubscribeUpdate())

	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	t.Cleanup(queue.ShutDown)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	require.NoError(t, src.Start(ctx, queue))

	everyApplication := []reconcile.Request{
		{NamespacedName: types.NamespacedName{Namespace: "foo", Name: "first"}},
		{NamespacedName: types.NamespacedName{Namespace: "bar", Name: "second"}},
	}

	drain := func(t *testing.T) []reconcile.Request {
		t.Helper()

		require.Eventually(t, func() bool { return queue.Len() == len(everyApplication) }, time.Second, 10*time.Millisecond)

		requests := make([]reconcile.Request, 0, len(everyApplication))
		for range everyApplication {
			request, _ := queue.Get()
			queue.Done(request)
			requests = append(requests, request)
		}

		return requests
	}

	require.NoError(t, applications.Set(addonutils.Values{
		"applications": map[string]any{"update": map[string]any{"mode": "Manual"}},
	}))
	assert.ElementsMatch(t, everyApplication, drain(t), "a change of the global applications settings must requeue every application")

	changed := helpers.DefaultDeckhouseSettings()
	changed.Update.Mode = v1alpha2.UpdateModeManual.String()
	deckhouse.Set(changed)
	assert.ElementsMatch(t, everyApplication, drain(t), "a change of the deckhouse update settings must requeue every application")
}
