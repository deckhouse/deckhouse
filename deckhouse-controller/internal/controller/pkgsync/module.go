// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pkgsync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// placement is where a module's package comes from, as the sync derives it.
type placement struct {
	repository string
	version    string
	dev        bool
	embedded   bool
}

// placed reports whether any source claims the module; the zero placement means none does.
func (p placement) placed() bool {
	return p != placement{}
}

// syncModules brings every Module in line with where its package comes from and with its
// module config. A module the image ships, a pull override pins or a deployed release
// installed gets its spec and annotations; the config fields mirror the ModuleConfig and are
// cleared when it is gone; a module nothing backs any more is deleted. The conditions the
// old stack left without a reason get one, so the object passes the v1alpha2 schema on its
// next status write.
func (s *syncer) syncModules(ctx context.Context) error {
	placements, err := s.resolvePlacements(ctx)
	if err != nil {
		return fmt.Errorf("resolve module placements: %w", err)
	}

	configs, err := s.resolveModuleConfigs(ctx)
	if err != nil {
		return fmt.Errorf("resolve module configs: %w", err)
	}

	existing := new(v1alpha2.ModuleList)
	if err := s.reader.List(ctx, existing); err != nil {
		return fmt.Errorf("list modules: %w", err)
	}

	tracked := make(map[string]struct{}, len(existing.Items))

	for idx := range existing.Items {
		module := &existing.Items[idx]
		tracked[module.Name] = struct{}{}

		place := placements[module.Name]
		if !place.placed() && s.disposable(module) {
			s.logger.Info("module is not backed by a package, delete it", slog.String("name", module.Name))

			// a module already gone is the outcome asked for
			if err := s.writer.Delete(ctx, module); err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("delete module '%s': %w", module.Name, err)
			}

			continue
		}

		if err := s.patchModule(ctx, module, place, configs[module.Name]); err != nil {
			return err
		}

		if err := s.normalizeModuleStatus(ctx, module); err != nil {
			return err
		}
	}

	for name, place := range placements {
		if _, ok := tracked[name]; ok {
			continue
		}

		if err := s.createModule(ctx, name, place, configs[name]); err != nil {
			return err
		}
	}

	return nil
}

// resolvePlacements derives every module's package source, in precedence order: the running
// image, then a pull override, then the newest deployed release.
func (s *syncer) resolvePlacements(ctx context.Context) (map[string]placement, error) {
	embedded, err := s.embeddedPlacements()
	if err != nil {
		return nil, fmt.Errorf("resolve embedded placements: %w", err)
	}

	overridden, err := s.overridePlacements(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve override placements: %w", err)
	}

	released, err := s.releasePlacements(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve release placements: %w", err)
	}

	// the first source claiming a name keeps it, so a lower-precedence source never overwrites a higher one
	placements := make(map[string]placement, len(embedded)+len(overridden)+len(released))

	for _, source := range []map[string]placement{embedded, overridden, released} {
		for name, place := range source {
			if _, ok := placements[name]; !ok {
				placements[name] = place
			}
		}
	}

	return placements, nil
}

// embeddedPlacements returns a placement for every module the running image ships. A
// definition that does not parse is a build defect and fails the sync: dropping the module
// instead would delete a running one as unbacked.
func (s *syncer) embeddedPlacements() (map[string]placement, error) {
	entries, err := os.ReadDir(s.embeddedModulesDir)
	if err != nil {
		return nil, fmt.Errorf("read embedded modules dir: %w", err)
	}

	// an embedded module carries the running Deckhouse version reduced to major.minor.patch, the
	// same version the version pass names its ModulePackageVersion after
	version := app.EmbeddedPackageVersion(s.deckhouseVersion)

	placements := make(map[string]placement, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() || slices.Contains(app.DummyModules, entry.Name()) {
			continue
		}

		name, err := embeddedModuleName(filepath.Join(s.embeddedModulesDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("embedded module '%s': %w", entry.Name(), err)
		}

		placements[name] = placement{repository: repositoryNameEmbedded, version: version, embedded: true}
	}

	return placements, nil
}

// embeddedModuleName names the module a directory of the embedded modules dir ships: the
// definition names it, and a directory without one is named after itself without the weight
// prefix, the way the module loader does.
func embeddedModuleName(dir string) (string, error) {
	def, err := loader.LoadEmbeddedDefinition(dir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("load definition: %w", err)
		}

		return moduleNameFromDirName(filepath.Base(dir)), nil
	}

	if def.Name == "" {
		return moduleNameFromDirName(filepath.Base(dir)), nil
	}

	return def.Name, nil
}

// moduleNameFromDirName strips the "<weight>-" prefix of an embedded module directory.
func moduleNameFromDirName(dirName string) string {
	prefix, name, found := strings.Cut(dirName, "-")
	if !found || weightFromDirName(dirName) == 0 || prefix == "" {
		return dirName
	}

	return name
}

// overridePlacements pins every module a pull override names to the tag it carries. Any
// override counts, not only a ready one: the module it installed stays on disk while the
// registry is unreachable, and losing its object would hide a running module.
func (s *syncer) overridePlacements(ctx context.Context) (map[string]placement, error) {
	overrides := new(v1alpha2.ModulePullOverrideList)
	if err := s.reader.List(ctx, overrides); err != nil {
		return nil, fmt.Errorf("list module pull overrides: %w", err)
	}

	placements := make(map[string]placement, len(overrides.Items))

	for idx := range overrides.Items {
		mpo := &overrides.Items[idx]
		if !mpo.DeletionTimestamp.IsZero() {
			continue
		}

		repository, err := ModuleRepository(ctx, s.reader, mpo.Name)
		if err != nil {
			return nil, fmt.Errorf("resolve the repository of the module '%s': %w", mpo.Name, err)
		}

		if repository == "" {
			s.logger.Warn("no resource names the repository of the overridden module, skip its placement",
				slog.String("name", mpo.Name))

			continue
		}

		placements[mpo.Name] = placement{repository: repository, version: mpo.Spec.ImageTag, dev: true}
	}

	return placements, nil
}

// releasePlacements returns the newest deployed release per module, superseding the duplicates
// it passes: two releases both marked deployed is what a restart mid version bump leaves behind.
func (s *syncer) releasePlacements(ctx context.Context) (map[string]placement, error) {
	selector := client.MatchingLabels{v1alpha1.ModuleReleaseLabelStatus: v1alpha1.ModuleReleaseLabelDeployed}

	releases := new(v1alpha1.ModuleReleaseList)
	if err := s.reader.List(ctx, releases, selector); err != nil {
		return nil, fmt.Errorf("list module releases: %w", err)
	}

	deployed := make([]*v1alpha1.ModuleRelease, 0, len(releases.Items))
	versions := make(map[string]*semver.Version, len(releases.Items))

	for idx := range releases.Items {
		release := &releases.Items[idx]
		if release.Status.Phase != v1alpha1.ModuleReleasePhaseDeployed || !release.DeletionTimestamp.IsZero() {
			continue
		}

		// a release the version pass skipped as unparsable names no package version either
		version, err := semver.NewVersion(release.Spec.Version)
		if err != nil {
			s.logger.Warn("release version is not a semver, skip its placement",
				slog.String("release", release.Name), slog.String("version", release.Spec.Version), log.Err(err))

			continue
		}

		deployed = append(deployed, release)
		versions[release.Name] = version
	}

	// newest first, so every deployed release a module has after its first is superseded by it
	slices.SortFunc(deployed, func(a, b *v1alpha1.ModuleRelease) int {
		return versions[b.Name].Compare(versions[a.Name])
	})

	placements := make(map[string]placement, len(deployed))

	for _, release := range deployed {
		name := release.GetModuleName()

		// superseding is hygiene on its own: it runs even for modules a higher-precedence source owns
		if _, ok := placements[name]; ok {
			if err := s.supersedeRelease(ctx, release); err != nil {
				s.logger.Error("failed to supersede the deployed module release",
					slog.String("name", release.Name), log.Err(err))
			}

			continue
		}

		source := release.GetModuleSource()
		if source == "" {
			s.logger.Warn("deployed release has no module source, skip its placement",
				slog.String("release", release.Name))

			continue
		}

		placements[name] = placement{repository: RepositoryNameForSource(source), version: "v" + versions[release.Name].String()}
	}

	return placements, nil
}

// supersedeRelease marks a deployed release that a newer deployed one replaced.
func (s *syncer) supersedeRelease(ctx context.Context, release *v1alpha1.ModuleRelease) error {
	superseded := release.DeepCopy()
	superseded.Status.Phase = v1alpha1.ModuleReleasePhaseSuperseded
	superseded.Status.Message = ""
	superseded.Status.TransitionTime = metav1.NewTime(s.dc.GetClock().Now().UTC())

	if err := s.writer.Status().Patch(ctx, superseded, client.MergeFrom(release)); err != nil {
		return fmt.Errorf("patch module release status: %w", err)
	}

	return nil
}

// ModuleRepository resolves the repository a module pinned to a dev tag pulls from. A pull
// override carries no source, so the answer is looked up in order: the repository the Module
// already names, the module config's source, the source of the newest deployed release, then
// of the newest pending one, and the only module source offering the module. Empty means no
// resource names one.
func ModuleRepository(ctx context.Context, reader client.Reader, name string) (string, error) {
	module := new(v1alpha2.Module)
	if err := reader.Get(ctx, client.ObjectKey{Name: name}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			return "", fmt.Errorf("get module: %w", err)
		}
	} else if module.Spec.PackageRepositoryName != "" && module.Spec.PackageRepositoryName != repositoryNameEmbedded {
		return module.Spec.PackageRepositoryName, nil
	}

	config := new(v1alpha1.ModuleConfig)
	if err := reader.Get(ctx, client.ObjectKey{Name: name}, config); err != nil {
		if !apierrors.IsNotFound(err) {
			return "", fmt.Errorf("get module config: %w", err)
		}
	} else if config.Spec.Source != "" {
		return RepositoryNameForSource(config.Spec.Source), nil
	}

	releases := new(v1alpha1.ModuleReleaseList)
	if err := reader.List(ctx, releases, client.MatchingLabels{v1alpha1.ModuleReleaseLabelModule: name}); err != nil {
		return "", fmt.Errorf("list module releases: %w", err)
	}

	// newest first, so the first release of a phase is the one the module runs or waits for
	slices.SortFunc(releases.Items, func(a, b v1alpha1.ModuleRelease) int {
		return b.GetVersion().Compare(a.GetVersion())
	})

	for _, phase := range []string{v1alpha1.ModuleReleasePhaseDeployed, v1alpha1.ModuleReleasePhasePending} {
		for idx := range releases.Items {
			release := &releases.Items[idx]
			if release.Status.Phase != phase || release.GetModuleSource() == "" {
				continue
			}

			return RepositoryNameForSource(release.GetModuleSource()), nil
		}
	}

	sources := new(v1alpha1.ModuleSourceList)
	if err := reader.List(ctx, sources); err != nil {
		return "", fmt.Errorf("list module sources: %w", err)
	}

	if offering := sources.Offering(name); len(offering) == 1 {
		return RepositoryNameForSource(offering[0]), nil
	}

	return "", nil
}

// resolveModuleConfigs maps every live module config onto the module it configures.
func (s *syncer) resolveModuleConfigs(ctx context.Context) (map[string]*v1alpha1.ModuleConfig, error) {
	list := new(v1alpha1.ModuleConfigList)
	if err := s.reader.List(ctx, list); err != nil {
		return nil, fmt.Errorf("list module configs: %w", err)
	}

	configs := make(map[string]*v1alpha1.ModuleConfig, len(list.Items))

	for idx := range list.Items {
		conf := &list.Items[idx]

		// a config on its way out is the config controller's business, not the sync's
		if !conf.DeletionTimestamp.IsZero() {
			continue
		}

		configs[conf.Name] = conf
	}

	return configs, nil
}

// disposable reports whether nothing backs an unplaced module: it carries no package version,
// it is an embedded module the image stopped shipping and no real repository has taken over,
// or it is a downloaded module whose files are gone. A downloaded module still on disk stays:
// a pull override deleted without a rollback leaves its files in use until the next deploy.
func (s *syncer) disposable(module *v1alpha2.Module) bool {
	if module.Spec.PackageVersion == "" {
		return true
	}

	if module.IsEmbedded() || module.Spec.PackageRepositoryName == repositoryNameEmbedded {
		return module.IsEmbedded() && module.Spec.PackageRepositoryName == repositoryNameEmbedded
	}

	_, err := os.Stat(filepath.Join(s.downloadedModulesDir, module.Name))

	return errors.Is(err, os.ErrNotExist)
}

// createModule places a module the cluster does not carry yet.
func (s *syncer) createModule(ctx context.Context, name string, place placement, conf *v1alpha1.ModuleConfig) error {
	module := &v1alpha2.Module{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha2.ModuleGVK.GroupVersion().String(),
			Kind:       v1alpha2.ModuleKind,
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	applyDesired(module, place, conf)

	if err := s.writer.Create(ctx, module); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create module '%s': %w", name, err)
		}

		// something created the module between the list and this call, so converge it here:
		// the sync runs once, and an object left as the racing writer made it stays that way
		module = new(v1alpha2.Module)
		if err := s.reader.Get(ctx, client.ObjectKey{Name: name}, module); err != nil {
			return fmt.Errorf("get module '%s': %w", name, err)
		}

		return s.patchModule(ctx, module, place, conf)
	}

	s.logger.Debug("module created", slog.String("name", name), slog.String("version", place.version))

	return nil
}

// patchModule writes placement, annotations and config fields in one patch, and nothing when
// none drifted.
func (s *syncer) patchModule(ctx context.Context, module *v1alpha2.Module, place placement, conf *v1alpha1.ModuleConfig) error {
	patch := client.MergeFrom(module.DeepCopy())

	applyDesired(module, place, conf)

	data, err := patch.Data(module)
	if err != nil {
		return fmt.Errorf("build patch for the module '%s': %w", module.Name, err)
	}

	if string(data) == "{}" {
		return nil
	}

	if err := s.writer.Patch(ctx, module, client.RawPatch(patch.Type(), data)); err != nil {
		return fmt.Errorf("patch module '%s': %w", module.Name, err)
	}

	s.logger.Debug("module synced", slog.String("name", module.Name), slog.String("version", place.version))

	return nil
}

// applyDesired writes the placement, its annotations and the config fields onto the module.
func applyDesired(module *v1alpha2.Module, place placement, conf *v1alpha1.ModuleConfig) {
	// an unplaced module keeps the spec another writer gave it; only disposable decides its fate
	if place.placed() {
		module.Spec.PackageRepositoryName = place.repository
		module.Spec.PackageVersion = place.version
	}

	// the annotations, not the spec, route a module to the image or to a dev tag, so both are
	// reconciled both ways
	setAnnotation(module, v1alpha2.ModuleAnnotationEmbedded, place.embedded)
	setAnnotation(module, v1alpha2.ModuleAnnotationDev, place.dev)

	// the config fields belong to the ModuleConfig: a module without one carries none
	if conf == nil {
		conf = new(v1alpha1.ModuleConfig)
	}

	module.Spec.Settings = conf.Spec.Settings
	module.Spec.SettingsVersion = conf.Spec.Version
	module.Spec.Maintenance = conf.Spec.Maintenance
	module.Spec.Enabled = conf.Spec.Enabled
	module.Spec.UpdatePolicy = conf.Spec.UpdatePolicy
}

// setAnnotation marks the key true or drops it, allocating the map on the first mark.
func setAnnotation(module *v1alpha2.Module, key string, set bool) {
	if !set {
		delete(module.Annotations, key)

		return
	}

	if module.Annotations == nil {
		module.Annotations = make(map[string]string)
	}

	module.Annotations[key] = "true"
}

// normalizeModuleStatus gives every condition the old stack wrote without a reason one. The
// v1alpha2 schema requires a reason, and a stored condition without the key fails validation
// as soon as any status write touches the object.
func (s *syncer) normalizeModuleStatus(ctx context.Context, module *v1alpha2.Module) error {
	original := module.DeepCopy()

	changed := false
	for idx := range module.Status.Conditions {
		cond := &module.Status.Conditions[idx]
		if cond.Reason != "" {
			continue
		}

		cond.Reason = legacyConditionReason(cond)
		changed = true
	}

	if !changed {
		return nil
	}

	if err := s.writer.Status().Patch(ctx, module, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch module status '%s': %w", module.Name, err)
	}

	s.logger.Debug("module conditions normalized", slog.String("name", module.Name))

	return nil
}

// legacyConditionReason names the reason a v1alpha1 writer left implicit: the True state of a
// condition is its own reason, a disabled state is Disabled, everything else is Unknown.
func legacyConditionReason(cond *metav1.Condition) string {
	switch cond.Status {
	case metav1.ConditionTrue:
		switch cond.Type {
		case v1alpha1.ModuleConditionEnabledByModuleConfig, v1alpha1.ModuleConditionEnabledByModuleManager:
			return v1alpha1.ModuleReasonEnabled
		case v1alpha1.ModuleConditionIsReady:
			return v1alpha1.ModuleReasonReady
		case v1alpha1.ModuleConditionLastReleaseDeployed:
			return v1alpha1.ModuleReasonDeployed
		case v1alpha1.ModuleConditionIsOverridden:
			return v1alpha1.ModuleReasonOverridden
		}
	case metav1.ConditionFalse:
		switch cond.Type {
		case v1alpha1.ModuleConditionEnabledByModuleConfig, v1alpha1.ModuleConditionEnabledByModuleManager:
			return v1alpha1.ModuleReasonDisabled
		}
	}

	return v1alpha1.ModuleReasonUnknown
}
