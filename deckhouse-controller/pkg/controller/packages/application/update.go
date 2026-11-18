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

// updateSettings resolves the update settings the application follows. The mode and the windows
// are its own spec.update alone, Manual without it. The notification and the blocking on alerts
// are the deckhouse update settings, since an application cannot set them. Blocks until the
// deckhouse settings first arrive.
func (r *reconciler) updateSettings(app *v1alpha1.Application) releaseupdater.Settings {
	deckhouse := r.deckhouseSettings.GetUpdateSettings()

	// The disruption approval is a Deckhouse release matter, so it is not passed on.
	settings := releaseupdater.Settings{
		Mode:               app.UpdateMode(),
		NotificationConfig: deckhouse.NotificationConfig,
		BlockOnAlerts:      deckhouse.BlockOnAlerts,
	}

	if app.Spec.Update != nil {
		settings.Windows = toUpdateWindows(app.Spec.Update.Windows)
	}

	return settings
}

// applyNow reports whether the application asks to skip the update windows for the version the engine runs
// (v1alpha1.ApplicationAnnotationApplyNow).
func applyNow(app *v1alpha1.Application) bool {
	version := app.Annotations[v1alpha1.ApplicationAnnotationApplyNow]

	return version != "" && version == app.TargetVersion()
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

// requeueOnSettingsChange requeues every application whenever the deckhouse update settings change,
// so the change is merged into the update settings each of them hands to the runtime. The
// subscription is taken before the controller starts, so a change that lands in between is still
// pending on its channel.
func (r *reconciler) requeueOnSettingsChange(changed <-chan struct{}) source.Func {
	return func(ctx context.Context, queue workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
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
