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

	"k8s.io/apimachinery/pkg/api/equality"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status"
	appupdate "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/update"
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

// applyNow reports whether the application asks to skip the update windows for the version the engine runs
// (v1alpha1.ApplicationAnnotationApplyNow).
func applyNow(app *v1alpha1.Application) bool {
	version := app.Annotations[v1alpha1.ApplicationAnnotationApplyNow]

	return version != "" && version == appupdate.Version(app)
}

// updateInputChanged passes an update of the status the automatic update steps on: the installed version,
// UpdateInstalled or Ready, which the status service writes and which bump no generation.
func updateInputChanged() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldApp, ok := e.ObjectOld.(*v1alpha1.Application)
			if !ok {
				return false
			}

			newApp, ok := e.ObjectNew.(*v1alpha1.Application)
			if !ok {
				return false
			}

			return installedVersion(oldApp) != installedVersion(newApp) ||
				conditionFlipped(oldApp, newApp, status.ConditionUpdateInstalled) ||
				conditionFlipped(oldApp, newApp, status.ConditionReady)
		},
	}
}

// conditionFlipped reports whether the condition became True or stopped being True.
func conditionFlipped(oldApp, newApp *v1alpha1.Application, condType string) bool {
	return apimeta.IsStatusConditionTrue(oldApp.Status.Conditions, condType) !=
		apimeta.IsStatusConditionTrue(newApp.Status.Conditions, condType)
}

// installedVersion returns status.currentVersion of the application, empty before the first install.
func installedVersion(app *v1alpha1.Application) string {
	if app.Status.CurrentVersion == nil {
		return ""
	}

	return app.Status.CurrentVersion.Version
}

// planInputChanged passes an update of a package version that changes the update plan; the installed lists the
// controller itself writes to the version's status do not pass.
func planInputChanged() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldApv, ok := e.ObjectOld.(*v1alpha1.ApplicationPackageVersion)
			if !ok {
				return false
			}

			newApv, ok := e.ObjectNew.(*v1alpha1.ApplicationPackageVersion)
			if !ok {
				return false
			}

			return oldApv.IsDraft() != newApv.IsDraft() ||
				oldApv.Labels[v1alpha1.PackageLabelExistInRegistry] != newApv.Labels[v1alpha1.PackageLabelExistInRegistry] ||
				!equality.Semantic.DeepEqual(oldApv.Status.PackageMetadata, newApv.Status.PackageMetadata)
		},
	}
}

// requestsForVersion returns a request for every application of the version's package, whose plan the version may change.
func (r *reconciler) requestsForVersion(ctx context.Context, obj client.Object) []reconcile.Request {
	repository := obj.GetLabels()[v1alpha1.PackageLabelRepository]
	pkg := obj.GetLabels()[v1alpha1.PackageLabelPackage]

	apps := new(v1alpha1.ApplicationList)
	if err := r.client.List(ctx, apps); err != nil {
		r.logger.Warn("failed to list applications to recompute their update plan", log.Err(err))

		return nil
	}

	var requests []reconcile.Request
	for _, app := range apps.Items {
		if app.Spec.PackageRepositoryName == repository && app.Spec.PackageName == pkg {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&app)})
		}
	}

	return requests
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
		Mode:               appupdate.ParseMode(policy.Mode),
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
