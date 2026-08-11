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

package controller

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"

	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	pkgmodules "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	pkgruntime "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// embeddedLoadWorkers caps how many embedded modules are read from disk concurrently.
	embeddedLoadWorkers = 8

	// embeddedRepositoryName stands for the Deckhouse image itself and resolves to no
	// PackageRepository — unlike `deckhouse`, which is a name a real repository may take.
	embeddedRepositoryName = "embedded"
)

// dummyModules are modules that should be skipped.
var dummyModules = []string{
	"000-common",
	"007-registrypackages",
}

// placement is where a module's package comes from, as the bootstrap derives it.
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

// resolvePlacements derives every module's package source, in precedence order: the running image,
// then a ready pull override, then the newest deployed release.
func (c *Controller) resolvePlacements(ctx context.Context) (map[string]placement, error) {
	embedded, err := c.embeddedPlacements(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve embedded placements: %w", err)
	}

	overridden, err := c.overridePlacements(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve override placements: %w", err)
	}

	released, err := c.releasePlacements(ctx)
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

// embeddedPlacements returns a placement for every module the running image ships.
func (c *Controller) embeddedPlacements(ctx context.Context) (map[string]placement, error) {
	embeddedDir := app.EmbeddedModulesDir

	c.logger.Debug("load embedded modules", slog.String("path", embeddedDir))

	entries, err := os.ReadDir(embeddedDir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	// only a definition names its module; each goroutine owns one slot, so the names need no lock
	names := make([]string, len(entries))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(embeddedLoadWorkers)

	for i, entry := range entries {
		if !entry.IsDir() || slices.Contains(dummyModules, entry.Name()) {
			continue
		}

		g.Go(func() error {
			// bail out before any work if another module already failed or the caller cancelled
			if err := ctx.Err(); err != nil {
				return err
			}

			c.logger.Debug("load embedded module", slog.String("name", entry.Name()))

			conf, err := loader.LoadEmbeddedConf(ctx, embeddedDir+"/"+entry.Name(), c.logger)
			if err != nil {
				return fmt.Errorf("load embedded conf: %w", err)
			}

			names[i] = conf.Definition.Name

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	placements := make(map[string]placement, len(names))

	for _, name := range names {
		if name == "" {
			continue
		}

		// an embedded module carries the running Deckhouse version — the runtime's edition version verbatim
		placements[name] = placement{repository: embeddedRepositoryName, version: app.Version, embedded: true}
	}

	return placements, nil
}

// overridePlacements pins every module a ready pull override names to the tag it carries.
func (c *Controller) overridePlacements(ctx context.Context) (map[string]placement, error) {
	cli := c.ctrl.GetClient()

	overrides := new(v1alpha2.ModulePullOverrideList)
	if err := cli.List(ctx, overrides); err != nil {
		return nil, fmt.Errorf("list module overrides: %w", err)
	}

	placements := make(map[string]placement, len(overrides.Items))

	for _, mpo := range overrides.Items {
		if !mpo.DeletionTimestamp.IsZero() || mpo.Status.Message != v1alpha1.ModulePullOverrideMessageReady {
			continue
		}

		// the v1alpha1 module is read only for its source, which the override does not carry
		module := new(v1alpha1.Module)
		if err := cli.Get(ctx, client.ObjectKey{Name: mpo.Name}, module); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("get module '%s': %w", mpo.Name, err)
			}

			c.logger.Info("module not exist, skip restoring module pull override", slog.String("name", mpo.Name))

			continue
		}

		placements[mpo.Name] = placement{repository: module.Properties.Source, version: mpo.Spec.ImageTag, dev: true}
	}

	return placements, nil
}

// releasePlacements returns the newest deployed release per module, superseding the duplicates it
// passes — two releases both marked deployed is what a restart mid version bump leaves behind.
func (c *Controller) releasePlacements(ctx context.Context) (map[string]placement, error) {
	selector := client.MatchingLabels{v1alpha1.ModuleReleaseLabelStatus: v1alpha1.ModuleReleaseLabelDeployed}

	releases := new(v1alpha1.ModuleReleaseList)
	if err := c.ctrl.GetClient().List(ctx, releases, selector); err != nil {
		return nil, fmt.Errorf("list module releases: %w", err)
	}

	// newest first, so every deployed release a module has after its first is superseded by it
	slices.SortFunc(releases.Items, func(a, b v1alpha1.ModuleRelease) int {
		return b.GetVersion().Compare(a.GetVersion())
	})

	placements := make(map[string]placement, len(releases.Items))

	for _, release := range releases.Items {
		if release.Status.Phase != v1alpha1.ModuleReleasePhaseDeployed || !release.DeletionTimestamp.IsZero() {
			continue
		}

		name := release.GetModuleName()

		// superseding is hygiene on its own: it runs even for modules a higher-precedence source owns
		if _, ok := placements[name]; ok {
			if err := c.supersedeRelease(ctx, &release); err != nil {
				c.logger.Error("failed to supersede the deployed module release",
					slog.String("name", release.GetName()), log.Err(err))
			}

			continue
		}

		placements[name] = placement{repository: release.GetModuleSource(), version: release.GetModuleVersion()}
	}

	return placements, nil
}

// supersedeRelease marks a deployed release that a newer deployed one replaced.
func (c *Controller) supersedeRelease(ctx context.Context, release *v1alpha1.ModuleRelease) error {
	superseded := release.DeepCopy()
	superseded.Status.Phase = v1alpha1.ModuleReleasePhaseSuperseded
	superseded.Status.Message = ""
	superseded.Status.TransitionTime = metav1.NewTime(c.dc.GetClock().Now().UTC())

	if err := c.ctrl.GetClient().Status().Patch(ctx, superseded, client.MergeFrom(release)); err != nil {
		return fmt.Errorf("patch module release status: %w", err)
	}

	return nil
}

// syncModules brings every module in line with its placement and its config, and returns the
// survivors carrying what was written — see package-flow.md for the placement rules.
func (c *Controller) syncModules(ctx context.Context, placements map[string]placement) ([]v1alpha2.Module, error) {
	configs, err := c.resolveModuleConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve module configs: %w", err)
	}

	// The manager's cached client serves a write from this same pass only once its watch event lands,
	// so this one snapshot, which every decision below is taken against, comes from the API server.
	existing := new(v1alpha2.ModuleList)
	if err := c.ctrl.GetAPIReader().List(ctx, existing); err != nil {
		return nil, fmt.Errorf("list modules: %w", err)
	}

	surviving := make([]v1alpha2.Module, 0, len(existing.Items)+len(placements))
	tracked := make(map[string]struct{}, len(existing.Items))

	for i := range existing.Items {
		module := &existing.Items[i]
		tracked[module.Name] = struct{}{}

		place := placements[module.Name]
		if !place.placed() && disposable(module) {
			c.logger.Info("module is not backed by a package, delete it", slog.String("name", module.Name))

			// a module already gone is the outcome asked for
			if err := c.ctrl.GetClient().Delete(ctx, module); err != nil && !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("delete module '%s': %w", module.Name, err)
			}

			continue
		}

		if err := c.patchModule(ctx, module, place, configs[module.Name]); err != nil {
			return nil, err
		}

		surviving = append(surviving, *module)
	}

	for name, place := range placements {
		if _, ok := tracked[name]; ok {
			continue
		}

		module, err := c.createModule(ctx, name, place, configs[name])
		if err != nil {
			return nil, err
		}

		surviving = append(surviving, *module)
	}

	return surviving, nil
}

// resolveModuleConfigs maps every live module config onto the module it configures.
func (c *Controller) resolveModuleConfigs(ctx context.Context) (map[string]*v1alpha1.ModuleConfig, error) {
	list := new(v1alpha1.ModuleConfigList)
	if err := c.ctrl.GetClient().List(ctx, list); err != nil {
		return nil, fmt.Errorf("list module configs: %w", err)
	}

	configs := make(map[string]*v1alpha1.ModuleConfig, len(list.Items))

	for i := range list.Items {
		conf := &list.Items[i]

		// a config on its way out is the config controller's business, not the bootstrap's
		if !conf.DeletionTimestamp.IsZero() {
			continue
		}

		configs[conf.Name] = conf
	}

	return configs, nil
}

// disposable reports whether nothing backs an unplaced module: it carries no package version, or it
// is an embedded module the image stopped shipping and no real repository has taken it over.
func disposable(module *v1alpha2.Module) bool {
	return module.Spec.PackageVersion == "" ||
		(module.IsEmbedded() && module.Spec.PackageRepositoryName == embeddedRepositoryName)
}

// createModule places a module the cluster does not carry yet.
func (c *Controller) createModule(ctx context.Context, name string, place placement, conf *v1alpha1.ModuleConfig) (*v1alpha2.Module, error) {
	module := &v1alpha2.Module{ObjectMeta: metav1.ObjectMeta{Name: name}}
	applyDesired(module, place, conf)

	if err := c.ctrl.GetClient().Create(ctx, module); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("create module '%s': %w", name, err)
		}

		// something created the module between the list and this call; the next start moves it if it drifted
		module = new(v1alpha2.Module)
		if err := c.ctrl.GetAPIReader().Get(ctx, client.ObjectKey{Name: name}, module); err != nil {
			return nil, fmt.Errorf("get module '%s': %w", name, err)
		}
	}

	return module, nil
}

// patchModule writes placement, annotations and settings in one patch, and nothing when none drifted.
func (c *Controller) patchModule(ctx context.Context, module *v1alpha2.Module, place placement, conf *v1alpha1.ModuleConfig) error {
	patch := client.MergeFrom(module.DeepCopy())

	applyDesired(module, place, conf)

	data, err := patch.Data(module)
	if err != nil {
		return fmt.Errorf("build patch for the module '%s': %w", module.Name, err)
	}

	if string(data) == "{}" {
		return nil
	}

	if err := c.ctrl.GetClient().Patch(ctx, module, client.RawPatch(patch.Type(), data)); err != nil {
		return fmt.Errorf("patch module '%s': %w", module.Name, err)
	}

	c.logger.Debug("module synced", slog.String("name", module.Name), slog.String("version", place.version))

	return nil
}

// applyDesired writes the placement, its annotations and the config's settings onto the module.
func applyDesired(module *v1alpha2.Module, place placement, conf *v1alpha1.ModuleConfig) {
	// an unplaced module keeps the spec another writer gave it — only disposable decides its fate
	if place.placed() {
		module.Spec.PackageRepositoryName = place.repository
		module.Spec.PackageVersion = place.version
	}

	// the annotation, not the spec, routes a module to the filesystem, so it is reconciled both ways
	if place.embedded {
		setAnnotation(module, v1alpha2.ModuleAnnotationEmbedded)
	} else {
		delete(module.Annotations, v1alpha2.ModuleAnnotationEmbedded)
	}

	// the dev annotation is only ever set, as it always has been
	if place.dev {
		setAnnotation(module, v1alpha2.ModuleAnnotationDev)
	}

	if conf == nil {
		return
	}

	module.Spec.Settings = conf.Spec.Settings
	module.Spec.SettingsVersion = conf.Spec.Version
	module.Spec.Maintenance = conf.Spec.Maintenance
	module.Spec.Enabled = conf.Spec.Enabled
}

// setAnnotation marks the key true, allocating the map when the module carries no annotations.
func setAnnotation(module *v1alpha2.Module, key string) {
	if module.Annotations == nil {
		module.Annotations = make(map[string]string)
	}

	module.Annotations[key] = "true"
}

// loadModules hands every placed module to the package runtime, which starts its pipeline.
func (c *Controller) loadModules(ctx context.Context, modules []v1alpha2.Module) error {
	// one repository backs many modules, so each is resolved once
	remotes := make(map[string]registry.Remote)

	for i := range modules {
		module := &modules[i]

		if !module.DeletionTimestamp.IsZero() {
			c.logger.Debug("module is deleted, skip loading", slog.String("module", module.Name))
			continue
		}

		// an embedded module is on disk already and its repository resolves to nothing
		if module.IsEmbedded() {
			c.manager.UpdateEmbeddedModule(runtimeModule(module))

			continue
		}

		if module.Spec.PackageRepositoryName == "" {
			c.logger.Debug("module has no repository, skip loading", slog.String("module", module.Name))
			continue
		}

		remote, ok := remotes[module.Spec.PackageRepositoryName]
		if !ok {
			repo := new(v1alpha1.PackageRepository)
			if err := c.ctrl.GetClient().Get(ctx, client.ObjectKey{Name: module.Spec.PackageRepositoryName}, repo); err != nil {
				return fmt.Errorf("get package repository '%s' of the module '%s': %w",
					module.Spec.PackageRepositoryName, module.Name, err)
			}

			remote = registry.BuildRemote(repo)
			remotes[module.Spec.PackageRepositoryName] = remote
		}

		pkg := runtimeModule(module)
		pkg.Definition = pkgmodules.Definition{Name: module.Name, Version: module.Spec.PackageVersion}

		c.manager.UpdateModule(remote, pkg, false)
	}

	return nil
}

// runtimeModule is what the runtime needs of a module: its identity, settings and enabled intent.
func runtimeModule(module *v1alpha2.Module) pkgruntime.Module {
	return pkgruntime.Module{
		Name:            module.Name,
		Settings:        module.Spec.Settings.GetMap(),
		SettingsVersion: module.Spec.SettingsVersion,
		Maintenance:     module.Spec.Maintenance,
		Enabled:         module.Spec.Enabled,
	}
}
