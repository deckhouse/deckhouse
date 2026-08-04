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

package release

import (
	"context"
	"crypto/md5"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"go.opentelemetry.io/otel"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// handleDeployedRelease keeps an already deployed release consistent: it refreshes the
// module's last-release-deployed condition, serves a pending reinstall or registry change,
// ensures finalizers, labels, documentation and settings ownership, and prunes outdated
// releases. A module held by a pull override is left alone.
func (r *reconciler) handleDeployedRelease(ctx context.Context, mr *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "handleDeployedRelease")
	defer span.End()

	res := ctrl.Result{}

	pendingReleaseFound, err := r.hasOlderPendingRelease(ctx, mr)
	if err != nil {
		return res, err
	}

	// an older pending release will be skipped later in the cycle, but until it is the module
	// cannot be called fully deployed
	if !pendingReleaseFound {
		if err = r.updateModuleLastReleaseDeployedStatus(ctx, mr, "", "", true); err != nil {
			return res, fmt.Errorf("update module last release deployed status: %w", err)
		}
	}

	if mr.GetReinstall() {
		if err = r.applyRelease(ctx, mr, nil); err != nil {
			return res, fmt.Errorf("run release deploy: %w", err)
		}

		r.logger.Info("module release reloaded", slog.String("release", mr.GetName()))

		return res, nil
	}

	if len(mr.Annotations) == 0 {
		mr.Annotations = make(map[string]string, 1)
	}

	var needsUpdate bool

	if mr.GetIsUpdating() {
		needsUpdate = true

		if r.isModuleReady(ctx, mr.GetModuleName()) {
			mr.Annotations[v1alpha1.ModuleReleaseAnnotationIsUpdating] = "false"
		}
	}

	// at least one release for module source is deployed, add finalizer to prevent module source deletion
	source := new(v1alpha1.ModuleSource)
	if err = r.client.Get(ctx, client.ObjectKey{Name: mr.GetModuleSource()}, source); err != nil {
		r.logger.Error("failed to get module source", slog.String("module_source", mr.GetModuleSource()), log.Err(err))

		return res, fmt.Errorf("get module source: %w", err)
	}

	// check if RegistrySpecChanged annotation is set process it
	if _, set := mr.GetAnnotations()[v1alpha1.ModuleReleaseAnnotationRegistrySpecChanged]; set {
		// the version is unchanged, so the runtime would skip an unforced update
		r.logger.Info("apply new registry settings to module", slog.String("module", mr.GetModuleName()))
		r.updateModule(source, mr, true)

		// delete annotation and requeue
		delete(mr.ObjectMeta.Annotations, v1alpha1.ModuleReleaseAnnotationRegistrySpecChanged)
		needsUpdate = true
	}

	// add finalizer
	if !controllerutil.ContainsFinalizer(mr, v1alpha1.ModuleReleaseFinalizerExistOnFs) {
		controllerutil.AddFinalizer(mr, v1alpha1.ModuleReleaseFinalizerExistOnFs)
		needsUpdate = true
	}

	if mr.Labels[v1alpha1.ModuleReleaseLabelStatus] != v1alpha1.ModuleReleaseLabelDeployed {
		if len(mr.ObjectMeta.Labels) == 0 {
			mr.ObjectMeta.Labels = make(map[string]string, 1)
		}

		mr.ObjectMeta.Labels[v1alpha1.ModuleReleaseLabelStatus] = v1alpha1.ModuleReleaseLabelDeployed
		needsUpdate = true
	}

	if needsUpdate {
		if err = r.client.Update(ctx, mr); err != nil {
			r.logger.Error("failed to update module release", slog.String("release", mr.GetName()), log.Err(err))

			return res, fmt.Errorf("update module release: %w", err)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	if !controllerutil.ContainsFinalizer(source, v1alpha1.ModuleSourceFinalizerReleaseExists) {
		controllerutil.AddFinalizer(source, v1alpha1.ModuleSourceFinalizerReleaseExists)
		if err = r.client.Update(ctx, source); err != nil {
			r.logger.Error("failed to add finalizer to module source", slog.String("module_source", mr.GetModuleSource()), log.Err(err))

			return res, fmt.Errorf("add finalizer to module source: %w", err)
		}
	}

	// checks if the module release is overridden by modulepulloverride
	exists, err := utils.ModulePullOverrideExists(ctx, r.client, mr.GetModuleName())
	if err != nil {
		r.logger.Error("failed to get module pull override", slog.String("module", mr.GetModuleName()), log.Err(err))

		return res, fmt.Errorf("module pull override exists: %w", err)
	}

	if exists {
		r.logger.Debug("module is overridden, skip it", slog.String("module", mr.GetModuleName()))

		return res, nil
	}

	if err = r.ensureDocumentation(ctx, mr); err != nil {
		return res, err
	}

	r.logger.Debug("delete outdated releases for module", slog.String("module", mr.GetModuleName()))
	if err = r.deleteOutdatedModuleReleases(ctx, mr.GetModuleSource(), mr.GetModuleName()); err != nil {
		r.logger.Error("failed to delete outdated module releases", slog.String("module", mr.GetModuleName()), log.Err(err))

		return res, fmt.Errorf("delete outdated module releases: %w", err)
	}

	return res, r.ownModuleSettings(ctx, mr)
}

// hasOlderPendingRelease reports whether the module has a pending release older than the
// deployed one, which reconcile will skip but which still blocks a clean deployed verdict.
func (r *reconciler) hasOlderPendingRelease(ctx context.Context, mr *v1alpha1.ModuleRelease) (bool, error) {
	releases := new(v1alpha1.ModuleReleaseList)
	labelSelector := client.MatchingLabels{
		v1alpha1.ModuleReleaseLabelSource: mr.GetModuleSource(),
		v1alpha1.ModuleReleaseLabelModule: mr.GetModuleName(),
	}

	if err := r.client.List(ctx, releases, labelSelector); err != nil {
		return false, fmt.Errorf("list module releases: %w", err)
	}

	for i := range releases.Items {
		if releases.Items[i].Status.Phase == v1alpha1.ModuleReleasePhasePending && mr.GetVersion().GreaterThan(releases.Items[i].GetVersion()) {
			return true, nil
		}
	}

	return false, nil
}

// ensureDocumentation republishes the module's documentation from this release. It skips a
// disabled module and a module whose embedded copy is still shipped, since neither serves docs
// from the downloaded release.
func (r *reconciler) ensureDocumentation(ctx context.Context, mr *v1alpha1.ModuleRelease) error {
	module := new(v1alpha1.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: mr.GetModuleName()}, module); err != nil {
		r.logger.Error("failed to get module", slog.String("module", mr.GetModuleName()), log.Err(err))

		return fmt.Errorf("get module: %w", err)
	}

	// ensure documentation for any enabled module, regardless of how it is enabled
	// (by module config, by bundle or by an enabled script) - EnabledByModuleManager
	// reflects the effective enabled state, unlike EnabledByModuleConfig which is only
	// set for modules enabled explicitly via a ModuleConfig
	if !module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleManager, corev1.ConditionTrue) || module.IsEmbedded() {
		return nil
	}

	// Use mount point path: /modules/<module> (modules are mounted at /deckhouse/downloaded/modules/deployed/<module>)
	modulePath := filepath.Join("/modules/deployed", mr.GetModuleName())
	moduleVersion := "v" + mr.GetVersion().String()

	// the checksum identifies the built docs; without a label to read it from, derive one that at
	// least changes with the version
	moduleChecksum := mr.Labels[v1alpha1.ModuleReleaseLabelReleaseChecksum]
	if moduleChecksum == "" {
		moduleChecksum = fmt.Sprintf("%x", md5.Sum([]byte(moduleVersion)))
	}

	ownerRef := metav1.OwnerReference{
		APIVersion: v1alpha1.ModuleReleaseGVK.GroupVersion().String(),
		Kind:       v1alpha1.ModuleReleaseGVK.Kind,
		Name:       mr.GetName(),
		UID:        mr.GetUID(),
		Controller: new(true),
	}

	if err := utils.EnsureModuleDocumentation(ctx, r.client, mr.GetModuleName(), module.Properties.Source, moduleChecksum, moduleVersion, modulePath, ownerRef); err != nil {
		r.logger.Error("failed to ensure module documentation", slog.String("module", mr.GetModuleName()), log.Err(err))

		return fmt.Errorf("ensure module documentation: %w", err)
	}

	return nil
}

// ownModuleSettings points the module's settings definition at this release, so the settings
// are garbage collected with it. A module without a settings definition is not an error.
func (r *reconciler) ownModuleSettings(ctx context.Context, mr *v1alpha1.ModuleRelease) error {
	settings := new(v1alpha1.ModuleSettingsDefinition)
	if err := r.client.Get(ctx, client.ObjectKey{Name: mr.GetModuleName()}, settings); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get module settings: %w", err)
		}

		r.logger.Warn("module settings not found", slog.String("module", mr.GetModuleName()))

		return nil
	}

	settings.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: v1alpha1.ModuleReleaseGVK.GroupVersion().String(),
		Kind:       v1alpha1.ModuleReleaseGVK.Kind,
		Name:       mr.GetName(),
		UID:        mr.GetUID(),
		Controller: new(true),
	}}

	if err := r.client.Update(ctx, settings); err != nil {
		r.logger.Warn("failed to update module settings", slog.String("module", mr.GetModuleName()), log.Err(err))

		return fmt.Errorf("update: %w", err)
	}

	return nil
}

// deleteRelease removes the module from the filesystem and releases the release's finalizers.
// It first parks the release in Terminating and requeues, so the phase is durable before
// anything is uninstalled.
func (r *reconciler) handleDelete(ctx context.Context, mr *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	if mr.GetPhase() != v1alpha1.ModuleReleasePhaseTerminating {
		mr.Status.Phase = v1alpha1.ModuleReleasePhaseTerminating
		if err := r.client.Status().Update(ctx, mr); err != nil {
			r.logger.Warn("failed to set terminating to the release", slog.String("release", mr.GetName()), log.Err(err))

			return ctrl.Result{}, fmt.Errorf("update: %w", err)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	// The metric is already reset in the handleRelease function, so we can release the finalizer
	if controllerutil.ContainsFinalizer(mr, v1alpha1.ModuleReleaseFinalizerMetricsRegistered) {
		controllerutil.RemoveFinalizer(mr, v1alpha1.ModuleReleaseFinalizerMetricsRegistered)
		if err := r.client.Update(ctx, mr); err != nil {
			r.logger.Error("failed to remove metrics finalizer from module release", slog.String("release", mr.GetName()), log.Err(err))
			return ctrl.Result{}, err
		}
	}

	// the runtime disables the module and unmounts it, so nothing is left running off the
	// filesystem once RemoveModule's Disable -> Undeploy pair drains
	if mr.GetLabels()[v1alpha1.ModuleReleaseLabelStatus] == strings.ToLower(v1alpha1.ModuleReleasePhaseDeployed) {
		r.logger.Info("remove module from the package runtime", slog.String("module", mr.GetModuleName()))

		r.manager.RemoveModule(mr.GetModuleName())
	}

	if controllerutil.ContainsFinalizer(mr, v1alpha1.ModuleReleaseFinalizerExistOnFs) {
		controllerutil.RemoveFinalizer(mr, v1alpha1.ModuleReleaseFinalizerExistOnFs)
		if err := r.client.Update(ctx, mr); err != nil {
			r.logger.Error("failed to update module release", slog.String("release", mr.GetName()), log.Err(err))
			return ctrl.Result{}, fmt.Errorf("update: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

// deleteOutdatedModuleReleases finds and deletes all outdated releases of the module in
// Suspend, Skipped or Superseded phases, except for outdatedReleasesKeepCount most recent ones.
func (r *reconciler) deleteOutdatedModuleReleases(ctx context.Context, moduleSource, module string) error {
	releases := new(v1alpha1.ModuleReleaseList)
	labelSelector := client.MatchingLabels{
		v1alpha1.ModuleReleaseLabelSource: moduleSource,
		v1alpha1.ModuleReleaseLabelModule: module,
	}

	if err := r.client.List(ctx, releases, labelSelector); err != nil {
		r.logger.Error("failed to list all module releases", log.Err(err))

		return fmt.Errorf("list releases: %w", err)
	}

	type outdatedRelease struct {
		name    string
		version *semver.Version
	}

	// the selector already pins one module, but a source may serve several, so group by name
	outdatedReleases := make(map[string][]outdatedRelease)

	for _, mr := range releases.Items {
		switch mr.GetPhase() {
		case v1alpha1.ModuleReleasePhaseSuperseded, v1alpha1.ModuleReleasePhaseSuspended, v1alpha1.ModuleReleasePhaseSkipped:
			outdatedReleases[mr.Spec.ModuleName] = append(outdatedReleases[mr.Spec.ModuleName], outdatedRelease{
				name:    mr.GetName(),
				version: mr.GetVersion(),
			})
		}
	}

	for moduleName, outdated := range outdatedReleases {
		r.logger.Debug("found the following outdated releases for module", slog.String("name", moduleName), slog.Any("releases_list", outdated))

		if len(outdated) <= outdatedReleasesKeepCount {
			continue
		}

		// newest first, so the tail past the keep count is what goes
		slices.SortFunc(outdated, func(a, b outdatedRelease) int {
			return b.version.Compare(a.version)
		})

		for _, release := range outdated[outdatedReleasesKeepCount:] {
			if err := r.client.Delete(ctx, newModuleReleaseWithName(release.name)); err != nil && !apierrors.IsNotFound(err) {
				r.logger.Error("failed to delete outdated release", slog.String("outdated_release", release.name), log.Err(err))

				return fmt.Errorf("delete outdated release: %w", err)
			}

			r.logger.Info("cleaned up outdated release", slog.String("outdated_release", release.name), slog.String("module_name", moduleName))
		}
	}

	return nil
}
