// Copyright 2024 Flant JSC
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

package source

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"time"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/controller/pkgsync"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
)

// syncRegistrySettings checks if modules source registry settings were updated
// (comparing moduleSourceAnnotationRegistryChecksum annotation and the current registry spec)
// and update relevant module releases' openapi values files if it is the case
func (r *reconciler) syncRegistrySettings(ctx context.Context, source *v1alpha1.ModuleSource) error {
	marshaled, err := json.Marshal(source.Spec.Registry)
	if err != nil {
		return fmt.Errorf("marshal the '%s' module source registry spec: %w", source.Name, err)
	}

	currentChecksum := fmt.Sprintf("%x", md5.Sum(marshaled))

	// if no annotations - only set the current checksum value
	if len(source.ObjectMeta.Annotations) == 0 {
		source.ObjectMeta.Annotations = map[string]string{
			v1alpha1.ModuleSourceAnnotationRegistryChecksum: currentChecksum,
		}

		return nil
	}

	// if the annotation matches current checksum - there is nothing to do here
	if source.ObjectMeta.Annotations[v1alpha1.ModuleSourceAnnotationRegistryChecksum] == currentChecksum {
		return ErrSettingsNotChanged
	}

	// get related releases
	moduleReleases := new(v1alpha1.ModuleReleaseList)
	if err = r.client.List(ctx, moduleReleases, client.MatchingLabels{v1alpha1.ModuleReleaseLabelSource: source.Name}); err != nil {
		return fmt.Errorf("list module releases to update registry settings: %w", err)
	}

	for _, release := range moduleReleases.Items {
		if release.Status.Phase == v1alpha1.ModuleReleasePhaseDeployed {
			for _, ref := range release.GetOwnerReferences() {
				if ref.UID == source.UID && ref.Name == source.Name && ref.Kind == v1alpha1.ModuleSourceGVK.Kind {
					if len(release.ObjectMeta.Annotations) == 0 {
						release.ObjectMeta.Annotations = make(map[string]string)
					}

					release.ObjectMeta.Annotations[v1alpha1.ModuleReleaseAnnotationRegistrySpecChanged] = r.dc.GetClock().Now().UTC().Format(time.RFC3339)
					if err = r.client.Update(ctx, &release); err != nil {
						return fmt.Errorf("set RegistrySpecChanged annotation to the '%s' module release: %w", release.Name, err)
					}

					break
				}
			}
		}
	}

	source.ObjectMeta.Annotations[v1alpha1.ModuleSourceAnnotationRegistryChecksum] = currentChecksum

	return nil
}

func (r *reconciler) releaseExists(ctx context.Context, sourceName, moduleName, checksum string) (bool, error) {
	// image digest has 64 symbols, while label can have maximum 63 symbols, so make md5 sum here
	checksum = fmt.Sprintf("%x", md5.Sum([]byte(checksum)))

	moduleReleases := new(v1alpha1.ModuleReleaseList)
	if err := r.client.List(ctx, moduleReleases, client.MatchingLabels{v1alpha1.ModuleReleaseLabelModule: moduleName, v1alpha1.ModuleReleaseLabelReleaseChecksum: checksum}); err != nil {
		return false, fmt.Errorf("list module releases: %w", err)
	}
	if len(moduleReleases.Items) == 0 {
		r.logger.Debug(
			"no module release with checksum for the module of source",
			slog.String("checksum", checksum),
			slog.String("name", moduleName),
			slog.String("source_name", sourceName),
		)
		return false, nil
	}

	r.logger.Debug(
		"module release with checksum exists for the module of source",
		slog.String("checksum", checksum),
		slog.String("name", moduleName),
		slog.String("source_name", sourceName),
	)
	return true, nil
}

// releaseEnsureAllowed reports whether a release for the module may be ensured from
// the given source at all. It is a policy predicate over already-fetched data: it does
// not decide whether there is actually something to fetch, that concrete diff
// (checksum change, missing target release, incomplete update chain) is evaluated by
// the caller. It returns false to skip the module entirely for experimental,
// not-active-source, disabled or ambiguous-source reasons. The module is nil until it
// is installed; the config is nil until the operator writes one.
func (r *reconciler) releaseEnsureAllowed(
	source *v1alpha1.ModuleSource,
	name string,
	module *v1alpha2.Module,
	config *v1alpha1.ModuleConfig,
	metadata *v1alpha1.ModulePackageVersionStatusMetadata,
	meta *downloader.ModuleDownloadResult,
	moduleSourceNames []string,
	embeddedTargetModuleSource string) bool {
	// skip experimental modules when deckhouse does not allow them: the channel definition
	// tells the stage of the offered version, the package metadata the one of the installed
	experimental := (meta.ModuleDefinition != nil && meta.ModuleDefinition.IsExperimental()) ||
		(metadata != nil && metadata.Stage == v1alpha1.ExperimentalModuleStage)
	if experimental && !r.deckhouseSettings.ExperimentalModuleAllowed(name) {
		r.logger.Debug("experimental module not allowed, skip release ensure",
			slog.String("source_name", source.Name),
			slog.String("name", name))

		return false
	}

	// An embedded module keeps its embedded copy while it is shipped, so the active-source
	// check below would always skip it. Instead pre-stage the release from the source
	// resolved for migration so the module is already on the filesystem when the embedded
	// copy is dropped on upgrade. embeddedTargetModuleSource is the resolved source (operator's
	// ModuleConfig .spec.source, or the only available source); it is empty when the
	// source is undecided (several sources, none chosen) - a conflict handled in
	// processModules.
	if module != nil && module.IsEmbedded() {
		if embeddedTargetModuleSource == "" || embeddedTargetModuleSource != source.Name {
			return false
		}
	} else if active := activeModuleSource(module, config); active != "" && active != source.Name {
		// check the active source
		r.logger.Debug("source not active, skip module",
			slog.String("source_name", source.Name),
			slog.String("name", name))

		return false
	}

	// no config, or a config that leaves the decision to the bundle
	if config == nil || config.Spec.Enabled == nil {
		enabledByBundle := false
		if meta.ModuleDefinition != nil {
			enabledByBundle = meta.ModuleDefinition.Accessibility.IsEnabled(r.edition.Name, r.edition.Bundle)
		}

		if !enabledByBundle {
			return false
		}

		if len(moduleSourceNames) > 1 && source.Name != defaultModuleSourceName {
			return false
		}

		return true
	}

	// disabled by module config
	if !config.IsEnabled() {
		return false
	}

	// an enabled config that picks no source among several: the config controller reports
	// the conflict, and no source installs the module until the operator picks one
	if activeModuleSource(module, config) == "" && len(moduleSourceNames) > 1 {
		return false
	}

	return true
}

// activeModuleSource names the source the module comes from: the one the config picks, otherwise
// the one the installed module was placed from. Empty when neither decides.
func activeModuleSource(module *v1alpha2.Module, config *v1alpha1.ModuleConfig) string {
	if configured := pkgsync.ConfiguredModuleSource(config); configured != "" {
		return configured
	}

	if module == nil || module.IsEmbedded() {
		return ""
	}

	return pkgsync.SourceNameForRepository(module.Spec.PackageRepositoryName)
}

// getModule returns the module object, nil when the module has no object yet.
func (r *reconciler) getModule(ctx context.Context, name string) (*v1alpha2.Module, error) {
	module := new(v1alpha2.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: name}, module); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("get the '%s' module: %w", name, err)
	}

	return module, nil
}

// ensureAvailableModule keeps the object of a module nothing installed: created when missing,
// its repository is the one of the source the config picks or of the only offering source,
// its channel the one of its policy, its status the available or the conflict state. The
// returned object is the one the rest of the scan reads.
func (r *reconciler) ensureAvailableModule(ctx context.Context, name, channel string, config *v1alpha1.ModuleConfig, moduleSourceNames []string) (*v1alpha2.Module, error) {
	module := new(v1alpha2.Module)
	err := r.client.Get(ctx, client.ObjectKey{Name: name}, module)
	if apierrors.IsNotFound(err) {
		module = &v1alpha2.Module{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha2.ModuleGVK.GroupVersion().String(),
				Kind:       v1alpha2.ModuleKind,
			},
			ObjectMeta: metav1.ObjectMeta{Name: name},
		}

		err = r.client.Create(ctx, module)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("create the '%s' module: %w", name, err)
		}

		// another source scanning the module created the object meanwhile: converge that one
		if err != nil {
			module = new(v1alpha2.Module)
			err = r.client.Get(ctx, client.ObjectKey{Name: name}, module)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("get the '%s' module: %w", name, err)
	}

	// a deploy that raced this scan owns the object from now on
	if module.IsInstalled() {
		return module, nil
	}

	if err := r.placeAvailableModule(ctx, module, channel, config, moduleSourceNames); err != nil {
		return nil, err
	}

	return module, nil
}

// placeAvailableModule writes where a module nothing installed would come from and its state:
// the repository of the source the config picks or of the only offering source, the channel,
// and the available or the conflict state.
func (r *reconciler) placeAvailableModule(ctx context.Context, module *v1alpha2.Module, channel string, config *v1alpha1.ModuleConfig, moduleSourceNames []string) error {
	configured := pkgsync.ConfiguredModuleSource(config)

	patch := client.MergeFrom(module.DeepCopy())
	module.Spec.PackageRepositoryName = pkgsync.PickRepository(configured, moduleSourceNames)
	module.Spec.ReleaseChannel = channel

	data, err := patch.Data(module)
	if err != nil {
		return fmt.Errorf("build patch for the '%s' module: %w", module.Name, err)
	}

	if string(data) != "{}" {
		if err := r.client.Patch(ctx, module, client.RawPatch(patch.Type(), data)); err != nil {
			return fmt.Errorf("patch the '%s' module: %w", module.Name, err)
		}
	}

	conflict := pkgsync.HasRepositoryConflict(config != nil && config.IsEnabled(), configured, moduleSourceNames)

	err = ctrlutils.UpdateStatusWithRetry(ctx, r.client, module, func() error {
		module.ApplyNotInstalledState(conflict)

		return nil
	})
	if err != nil {
		return fmt.Errorf("update the '%s' module status: %w", module.Name, err)
	}

	return nil
}

// cleanAvailableModule takes the source away from a module it stopped offering. The object of a
// module nothing installed and no other source offers goes; one another source still offers is
// placed on what remains. An installed module is not touched.
func (r *reconciler) cleanAvailableModule(ctx context.Context, source *v1alpha1.ModuleSource, name string) error {
	module, err := r.getModule(ctx, name)
	if err != nil {
		return err
	}

	if module == nil || module.IsInstalled() {
		return nil
	}

	moduleSources := new(v1alpha1.ModuleSourceList)
	if err := r.client.List(ctx, moduleSources); err != nil {
		return fmt.Errorf("list module sources: %w", err)
	}

	// the status of this source lists the module until this scan is written
	moduleSourceNames := slices.DeleteFunc(moduleSources.Offering(name), func(moduleSourceName string) bool {
		return moduleSourceName == source.Name
	})

	if len(moduleSourceNames) == 0 {
		r.logger.Info("no source offers the module any more, delete it", slog.String("module_name", name), slog.String("source_name", source.Name))

		if err := r.client.Delete(ctx, module); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete the '%s' module: %w", name, err)
		}

		return nil
	}

	config, err := r.moduleConfig(ctx, name)
	if err != nil {
		return err
	}

	return r.placeAvailableModule(ctx, module, module.Spec.ReleaseChannel, config, moduleSourceNames)
}

// moduleConfig returns the module config, nil when the operator wrote none.
func (r *reconciler) moduleConfig(ctx context.Context, name string) (*v1alpha1.ModuleConfig, error) {
	config := new(v1alpha1.ModuleConfig)
	if err := r.client.Get(ctx, client.ObjectKey{Name: name}, config); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("get the '%s' module config: %w", name, err)
	}

	return config, nil
}

// listingModuleSources names the sources whose last scan lists the module, and the source being
// scanned, which has just pulled it and lists it only once this scan is written.
func (r *reconciler) listingModuleSources(ctx context.Context, source *v1alpha1.ModuleSource, name string) ([]string, error) {
	moduleSources := new(v1alpha1.ModuleSourceList)
	if err := r.client.List(ctx, moduleSources); err != nil {
		return nil, fmt.Errorf("list module sources: %w", err)
	}

	moduleSourceNames := moduleSources.Offering(name)
	if !slices.Contains(moduleSourceNames, source.Name) {
		moduleSourceNames = append(moduleSourceNames, source.Name)
		sort.Strings(moduleSourceNames)
	}

	return moduleSourceNames, nil
}

// ensureReleaseChannel records the channel the module follows.
func (r *reconciler) ensureReleaseChannel(ctx context.Context, module *v1alpha2.Module, channel string) error {
	if module.Spec.ReleaseChannel == channel {
		return nil
	}

	patch := client.MergeFrom(module.DeepCopy())
	module.Spec.ReleaseChannel = channel

	if err := r.client.Patch(ctx, module, patch); err != nil {
		return fmt.Errorf("patch the '%s' module: %w", module.Name, err)
	}

	return nil
}

// releaseChainToTargetComplete reports whether the ModuleReleases already present in
// the cluster form a continuous update sequence from the deployed release up to (and
// including) the target version. It returns true (nothing to bridge) when there is no
// deployed release yet or the target is not ahead of the deployed one - those cases
// are handled by the regular first-install / no-op flow. A gap yields false,
// signalling that the step-by-step fetch must run again.
//
// The gap rule is shared with the fetcher via isSequentialReleasePair, so this check
// reports "complete" for exactly the chains the step-by-step fetch would leave
// untouched (including legitimate non-adjacent jumps allowed by a release's from-to
// update spec). This equivalence is what keeps the checksum guard from re-opening the
// fetch on every reconcile for from-to modules.
func (r *reconciler) releaseChainToTargetComplete(ctx context.Context, moduleName, targetVersion string) (bool, error) {
	target, err := semver.NewVersion(targetVersion)
	if err != nil {
		return false, fmt.Errorf("parse target version %q: %w", targetVersion, err)
	}

	releaseList := new(v1alpha1.ModuleReleaseList)
	if err = r.client.List(ctx, releaseList, client.MatchingLabels{v1alpha1.ModuleReleaseLabelModule: moduleName}); err != nil {
		return false, fmt.Errorf("list module releases: %w", err)
	}

	// GetVersion (used below and inside isSequentialReleasePair) relies on
	// semver.MustParse, which panics on a malformed Spec.Version. This check now runs on
	// the steady-state path (checksum unchanged, target exists) for every installed
	// module, so validate every release up front: a single corrupt object must surface
	// as a handled error - the caller keeps the known state and only sets errorsExist,
	// so no fetch is triggered - instead of panicking the whole reconcile.
	for i := range releaseList.Items {
		release := &releaseList.Items[i]
		if _, err = semver.NewVersion(release.Spec.Version); err != nil {
			return false, fmt.Errorf("parse release %q version %q: %w", release.Name, release.Spec.Version, err)
		}
	}

	// find the highest deployed release; without one the regular first-install flow applies
	var deployed *v1alpha1.ModuleRelease
	for i := range releaseList.Items {
		release := &releaseList.Items[i]
		if release.GetPhase() != v1alpha1.ModuleReleasePhaseDeployed {
			continue
		}
		if deployed == nil || release.GetVersion().GreaterThan(deployed.GetVersion()) {
			deployed = release
		}
	}
	if deployed == nil || !target.GreaterThan(deployed.GetVersion()) {
		return true, nil
	}

	// collect the deployed release together with the releases in (deployed, target]
	// and verify they form a continuous update sequence up to the target
	chain := []*v1alpha1.ModuleRelease{deployed}
	for i := range releaseList.Items {
		release := &releaseList.Items[i]
		if version := release.GetVersion(); version.GreaterThan(deployed.GetVersion()) && !version.GreaterThan(target) {
			chain = append(chain, release)
		}
	}
	sort.Slice(chain, func(i, j int) bool {
		return chain[i].GetVersion().LessThan(chain[j].GetVersion())
	})

	// the target itself must be present as the endpoint of the chain
	if chain[len(chain)-1].GetVersion().Compare(target) != 0 {
		return false, nil
	}

	for i := 1; i < len(chain); i++ {
		if !isSequentialReleasePair(chain[i-1], chain[i]) {
			return false, nil
		}
	}

	return true, nil
}

// isSequentialReleasePair reports whether an update may proceed directly from the
// lower release to the higher one, without any release in between: either the versions
// are naturally adjacent (isUpdatingSequence) or the higher release declares a from-to
// transition rule that targets the higher release itself and admits the lower version.
// This is the single rule used both to decide whether the in-cluster chain is complete
// (releaseChainToTargetComplete) and to pick the starting point of the step-by-step
// fetch (ensureReleases), so the two never disagree about what counts as a gap.
//
// The from-to rule matches the one the release updater enforces before it deploys a
// jump (releaseupdater.getFirstCompliantRelease): a constraint bridges the gap only when
// it points at this very release (its major.minor equals "to") and covers the lower
// version (lower is within [from, to)). Keeping the two sides identical is what stops
// the source controller from reporting a chain complete that the updater then refuses to
// walk - the mismatch that left a module stuck in Pending with a from-to whose "to"
// overshoots the release's own minor.
func isSequentialReleasePair(lower, higher *v1alpha1.ModuleRelease) bool {
	lowerVersion, higherVersion := lower.GetVersion(), higher.GetVersion()
	if isUpdatingSequence(lowerVersion, higherVersion) {
		return true
	}

	// the from-to rule is declared on the constrained (higher/"to") release
	spec := higher.GetUpdateSpec()
	if spec == nil {
		return false
	}

	for _, constraint := range spec.Versions {
		if fromToBridges(lowerVersion, higherVersion, constraint) {
			return true
		}
	}

	return false
}

// fromToBridges reports whether a single from-to constraint lets an update jump directly
// onto the higher release from the lower version. It mirrors the release updater: the
// constraint must target the higher release itself (higher major.minor equals the "to"
// major.minor) and the lower version must fall in the half-open window [from, to). A
// constraint that only parses but matches neither - a "to" pointing past the higher
// release, or a window that does not cover the lower version - does not bridge.
func fromToBridges(lower, higher *semver.Version, constraint v1alpha1.UpdateConstraint) bool {
	to, err := semver.NewVersion(constraint.To)
	if err != nil {
		return false
	}
	if higher.Major() != to.Major() || higher.Minor() != to.Minor() {
		return false
	}

	from, err := semver.NewVersion(constraint.From)
	if err != nil {
		return false
	}

	return lower.Compare(from) >= 0 && lower.Compare(to) < 0
}

// defaultModuleSourceName is the name of the built-in OSS ModuleSource that ships
// with Deckhouse. When a module is offered by it alongside mirrors (e.g. the EE
// source), it is the canonical choice rather than an ambiguous conflict.
const defaultModuleSourceName = "deckhouse"

// resolveEmbeddedTargetModuleSource decides which source an embedded module should be
// pre-staged from while its embedded copy is still shipped, and whether the choice
// is a genuine conflict.
//
// Being offered by several sources is not automatically a conflict:
//   - an explicitly chosen source wins (if it still offers the module);
//   - the "Embedded" sentinel is not a real source, so "Embedded" + one real source
//     is not a conflict - the real source is used;
//   - the built-in "deckhouse" source is the canonical default and wins over mirrors
//     such as "deckhouse-upstream-ee".
//
// It is a conflict only when the chosen source no longer offers the module, or when
// several real, non-default sources offer it and none is selected.
func resolveEmbeddedTargetModuleSource(configuredModuleSource string, moduleSourceNames []string) (string, bool) {
	if configuredModuleSource != "" {
		if slices.Contains(moduleSourceNames, configuredModuleSource) {
			return configuredModuleSource, false
		}
		// the configured .spec.source does not (or no longer does) offer the module
		return "", true
	}

	// "Embedded" is a sentinel for the built-in copy, not a selectable source.
	candidates := make([]string, 0, len(moduleSourceNames))
	for _, source := range moduleSourceNames {
		if source != v1alpha1.ModuleSourceEmbedded {
			candidates = append(candidates, source)
		}
	}

	switch {
	case len(candidates) == 0:
		// nothing but the embedded copy is available - nothing to pre-stage, not a conflict
		return "", false
	case len(candidates) == 1:
		return candidates[0], false
	case slices.Contains(candidates, defaultModuleSourceName):
		return defaultModuleSourceName, false
	default:
		return "", true
	}
}

func (r *reconciler) updateModuleSourceStatusMessage(ctx context.Context, source *v1alpha1.ModuleSource, message string) error {
	err := utils.UpdateStatus(ctx, r.client, source, func(source *v1alpha1.ModuleSource) bool {
		source.Status.Phase = v1alpha1.ModuleSourcePhaseActive
		source.Status.SyncTime = metav1.NewTime(r.dc.GetClock().Now().UTC())
		source.Status.Message = message
		return true
	})
	if err != nil {
		return fmt.Errorf("update the '%s' module source status: %w", source.Name, err)
	}

	return nil
}
