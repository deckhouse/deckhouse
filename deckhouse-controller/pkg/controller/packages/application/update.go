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

package application

import (
	"context"
	"slices"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// deckhouseSettings hands out the deckhouse update settings, parsed outside the controller once per
// change of the deckhouse ModuleConfig.
type deckhouseSettings interface {
	GetUpdateSettings() releaseupdater.Settings
	SubscribeUpdate() <-chan struct{}
}

// applicationsSettings hands out global.applications.update, parsed outside the controller once per
// change of the global ModuleConfig.
type applicationsSettings interface {
	GetUpdateSettings() (releaseupdater.Settings, bool)
	SubscribeUpdate() <-chan struct{}
}

// updateSettings resolves the update settings the application follows:
//
//  1. its own spec.update, with the notification settings taken from the global level, since an
//     application cannot set them, the way a ModuleRelease takes them from the deckhouse settings;
//  2. without spec.update, the global level as a whole.
//
// The global level is global.applications.update when it is set, and the deckhouse update
// settings otherwise.
func (r *reconciler) updateSettings(app *v1alpha1.Application) releaseupdater.Settings {
	return mergeUpdateSettings(app.Spec.Update, r.globalUpdateSettings())
}

// globalUpdateSettings returns global.applications.update when it is set, and the deckhouse update
// settings otherwise. The latter blocks until the deckhouse settings first arrive.
func (r *reconciler) globalUpdateSettings() releaseupdater.Settings {
	if settings, ok := r.applicationsSettings.GetUpdateSettings(); ok {
		return settings
	}

	deckhouse := r.deckhouseSettings.GetUpdateSettings()

	// The disruption approval is a Deckhouse release matter, so it is not passed on.
	return releaseupdater.Settings{
		Mode:               deckhouse.Mode,
		Windows:            deckhouse.Windows,
		NotificationConfig: deckhouse.NotificationConfig,
		BlockOnAlerts:      deckhouse.BlockOnAlerts,
	}
}

// mergeUpdateSettings applies the update policy of an application over the global update
// settings. The policy has no notification settings, so those always come from the global ones.
func mergeUpdateSettings(policy *v1alpha1.ApplicationUpdate, global releaseupdater.Settings) releaseupdater.Settings {
	if policy == nil {
		return global
	}

	blockOnAlerts := releaseupdater.BlockOnAlerts{Severity: helpers.DefaultBlockOnAlertsSeverity}
	if policy.BlockOnAlerts != nil {
		blockOnAlerts.Enabled = policy.BlockOnAlerts.Enabled
		if policy.BlockOnAlerts.Severity != nil {
			blockOnAlerts.Severity = int(*policy.BlockOnAlerts.Severity)
		}
	}

	return releaseupdater.Settings{
		Mode:               v1alpha2.ParseUpdateMode(policy.Mode),
		Windows:            toUpdateWindows(policy.Windows),
		NotificationConfig: global.NotificationConfig,
		BlockOnAlerts:      blockOnAlerts,
	}
}

// toUpdateWindows copies the windows rather than aliasing them, because the result outlives this
// reconcile in the runtime.
func toUpdateWindows(windows []v1alpha1.ApplicationUpdateWindow) update.Windows {
	if len(windows) == 0 {
		return nil
	}

	out := make(update.Windows, 0, len(windows))
	for _, window := range windows {
		out = append(out, update.Window{
			From: window.From,
			To:   window.To,
			Days: slices.Clone(window.Days),
		})
	}

	return out
}

// requeueOnSettingsChange requeues every application whenever one of the global levels it falls
// back to changes, so the change is merged into the update settings each of them hands to the
// runtime. The subscriptions are taken before the controller starts, so a change that lands in
// between is still pending on its channel.
func (r *reconciler) requeueOnSettingsChange(changes ...<-chan struct{}) source.Func {
	return func(ctx context.Context, queue workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
		for _, changed := range changes {
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-changed:
						for _, request := range r.applicationRequests(ctx) {
							queue.Add(request)
						}
					}
				}
			}()
		}

		return nil
	}
}

// applicationRequests returns a request for every application.
func (r *reconciler) applicationRequests(ctx context.Context) []reconcile.Request {
	apps := new(v1alpha1.ApplicationList)
	if err := r.client.List(ctx, apps); err != nil {
		r.logger.Warn("failed to list applications to merge the update settings again", log.Err(err))

		return nil
	}

	requests := make([]reconcile.Request, 0, len(apps.Items))
	for _, app := range apps.Items {
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&app)})
	}

	return requests
}
