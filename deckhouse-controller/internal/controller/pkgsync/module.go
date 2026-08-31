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

// This file is the module pass of the sync: it converges the v1alpha2 Module
// resources to where every module's package comes from (the image, a ready
// pull override, the newest deployed release) and how it is configured (the
// live ModuleConfig). Identifiers carry the resource version: moduleV2 is the
// v1alpha2 Module this pass fills, moduleV1 is the legacy one - it shows up
// in a single read, the source of an overridden module.

import (
	"context"
	"fmt"
	"log/slog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
)

// syncModules makes the v1alpha2 Module resources match where every module's
// package actually comes from. The returned Modules carry what was written:
// the caller hands them straight to the package runtime without re-reading
// the cluster.
func (s *syncer) syncModules(ctx context.Context, embedded []embeddedModule, fromReleases map[string]Origin) ([]v1alpha2.Module, error) {
	fromImage := s.originsFromImage(embedded)

	fromPullOverrides, err := s.originsFromModulePullOverrides(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve origins from module pull overrides: %w", err)
	}

	// one origin per module; an earlier source beats a later one
	origins := mergeOrigins(fromImage, fromPullOverrides, fromReleases)

	// how are the modules configured?
	configs, err := s.liveModuleConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve module configs: %w", err)
	}

	// write it all into the Module resources
	modulesV2, err := s.writeModulesV2(ctx, origins, configs)
	if err != nil {
		return nil, fmt.Errorf("write modules: %w", err)
	}

	return modulesV2, nil
}

// originsFromImage returns an origin for every module the running image ships.
func (s *syncer) originsFromImage(embedded []embeddedModule) map[string]Origin {
	origins := make(map[string]Origin, len(embedded))

	for _, module := range embedded {
		// an embedded module carries the running Deckhouse version - the
		// runtime's edition version verbatim
		origins[module.def.Name] = Origin{
			RepositoryName: repositoryNameEmbedded,
			PackageVersion: s.deckhouseVersion,
			Embedded:       true,
		}
	}

	return origins
}

// originsFromModulePullOverrides pins every module a ready pull override
// names to the tag it carries. The resolver dies together with the
// ModulePullOverride deprecation.
func (s *syncer) originsFromModulePullOverrides(ctx context.Context) (map[string]Origin, error) {
	overrides := new(v1alpha2.ModulePullOverrideList)
	if err := s.reader.List(ctx, overrides); err != nil {
		return nil, fmt.Errorf("list module overrides: %w", err)
	}

	origins := make(map[string]Origin, len(overrides.Items))

	for _, mpo := range overrides.Items {
		if !mpo.DeletionTimestamp.IsZero() || mpo.Status.Message != v1alpha1.ModulePullOverrideMessageReady {
			continue
		}

		// the v1alpha1 Module is read only for its source, which the override does not carry
		moduleV1 := new(v1alpha1.Module)
		if err := s.reader.Get(ctx, client.ObjectKey{Name: mpo.Name}, moduleV1); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("get module '%s': %w", mpo.Name, err)
			}

			s.logger.Info("module not exist, skip restoring module pull override", slog.String("name", mpo.Name))

			continue
		}

		origin := Origin{RepositoryName: moduleV1.Properties.Source, PackageVersion: mpo.Spec.ImageTag, Dev: true}

		// a module without a source gives no repository to pull from; claiming
		// it here would only hide the release that does know one
		if !origin.Known() {
			s.logger.Info("module has no source, skip its pull override", slog.String("name", mpo.Name))

			continue
		}

		origins[mpo.Name] = origin
	}

	return origins, nil
}

// liveModuleConfigs maps every module config that is not being deleted onto
// the module it configures. The reader dies together with the ModuleConfig
// deprecation, along with the config block of fillModuleV2.
func (s *syncer) liveModuleConfigs(ctx context.Context) (map[string]*v1alpha1.ModuleConfig, error) {
	list := new(v1alpha1.ModuleConfigList)
	if err := s.reader.List(ctx, list); err != nil {
		return nil, fmt.Errorf("list module configs: %w", err)
	}

	configs := make(map[string]*v1alpha1.ModuleConfig, len(list.Items))

	for i := range list.Items {
		conf := &list.Items[i]

		// a config on its way out is the config controller's business, not the sync's
		if !conf.DeletionTimestamp.IsZero() {
			continue
		}

		configs[conf.Name] = conf
	}

	return configs, nil
}

// fillModuleV2 writes the origin, its annotations and the config's settings
// onto the module. Pure field mapping, no cluster access.
func fillModuleV2(moduleV2 *v1alpha2.Module, origin Origin, conf *v1alpha1.ModuleConfig) {
	// a module of unknown origin keeps the spec another writer gave it
	if origin.Known() {
		moduleV2.Spec.PackageRepositoryName = origin.RepositoryName
		moduleV2.Spec.PackageVersion = origin.PackageVersion
	}

	// the embedded annotation, not the spec, routes a module to the filesystem,
	// so a known origin reconciles it both ways; an unknown origin (a pure
	// config mirror) owns no annotations and leaves them be
	if origin.Known() {
		if origin.Embedded {
			markAnnotation(moduleV2, v1alpha2.ModuleAnnotationEmbedded)
		} else {
			delete(moduleV2.Annotations, v1alpha2.ModuleAnnotationEmbedded)
		}

		// the dev annotation is only ever set, as it always has been
		if origin.Dev {
			markAnnotation(moduleV2, v1alpha2.ModuleAnnotationDev)
		}
	}

	// the config block dies with the ModuleConfig deprecation, together with liveModuleConfigs
	if conf == nil {
		return
	}

	// spec.packageVersion is required by the Module schema, so config fields
	// cannot materialize the spec on their own: the API server would reject
	// such a write. They are filled once a version is there.
	if !origin.Known() && moduleV2.Spec.PackageVersion == "" {
		return
	}

	moduleV2.Spec.Settings = conf.Spec.Settings
	moduleV2.Spec.SettingsVersion = conf.Spec.Version
	moduleV2.Spec.Maintenance = conf.Spec.Maintenance
	moduleV2.Spec.Enabled = conf.Spec.Enabled
	moduleV2.Spec.UpdatePolicy = conf.Spec.UpdatePolicy

	// the config names the repository the module must come from, and it wins
	// over the origin: the origin only reports where the package came from
	// last. An embedded module keeps "embedded" - it ships in the image, and
	// no repository serves it.
	if conf.Spec.Source != "" && !moduleV2.IsEmbedded() {
		moduleV2.Spec.PackageRepositoryName = conf.Spec.Source
	}
}

// markAnnotation sets the marker key to "true", allocating the map when the
// module carries no annotations.
func markAnnotation(moduleV2 *v1alpha2.Module, key string) {
	if moduleV2.Annotations == nil {
		moduleV2.Annotations = make(map[string]string)
	}

	moduleV2.Annotations[key] = "true"
}

// writeModulesV2 brings every v1alpha2 Module in line with its origin and its
// config, and returns the survivors carrying what was written.
func (s *syncer) writeModulesV2(ctx context.Context, origins map[string]Origin, configs map[string]*v1alpha1.ModuleConfig) ([]v1alpha2.Module, error) {
	// one fresh snapshot, every decision below is taken against it
	existing := new(v1alpha2.ModuleList)
	if err := s.reader.List(ctx, existing); err != nil {
		return nil, fmt.Errorf("list modules: %w", err)
	}

	surviving := make([]v1alpha2.Module, 0, len(existing.Items)+len(origins))
	existingNames := make(map[string]struct{}, len(existing.Items))

	for i := range existing.Items {
		moduleV2 := &existing.Items[i]
		existingNames[moduleV2.Name] = struct{}{}

		kept, err := s.writeExistingModuleV2(ctx, moduleV2, origins[moduleV2.Name], configs[moduleV2.Name])
		if err != nil {
			return nil, err
		}

		if kept {
			surviving = append(surviving, *moduleV2)
		}
	}

	for name, origin := range origins {
		if _, ok := existingNames[name]; ok {
			continue
		}

		moduleV2, err := s.createModuleV2(ctx, name, origin, configs[name])
		if err != nil {
			return nil, err
		}

		surviving = append(surviving, *moduleV2)
	}

	return surviving, nil
}

// writeExistingModuleV2 patches the module to its origin and config, or
// deletes it when it is orphaned. It reports whether the module survived.
func (s *syncer) writeExistingModuleV2(ctx context.Context, moduleV2 *v1alpha2.Module, origin Origin, conf *v1alpha1.ModuleConfig) (bool, error) {
	// the module is orphaned when no source claims it and, on top of that,
	// no writer ever put a package version here - or only the image supplied
	// it and no longer ships it
	neverFilled := moduleV2.Spec.PackageVersion == ""
	imageOnly := moduleV2.IsEmbedded() && moduleV2.Spec.PackageRepositoryName == repositoryNameEmbedded

	if !origin.Known() && (neverFilled || imageOnly) {
		// while the old module stack owns the catalog, such modules are its
		// business: the sync neither deletes nor touches them
		if !s.deleteOrphans {
			s.logger.Debug("orphaned module, leave it to the module stack",
				slog.String("name", moduleV2.Name))

			return false, nil
		}

		s.logger.Info("orphaned module, delete it", slog.String("name", moduleV2.Name))

		// a module already gone is the outcome asked for
		if err := s.writer.Delete(ctx, moduleV2); err != nil && !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("delete module '%s': %w", moduleV2.Name, err)
		}

		return false, nil
	}

	if err := s.patchModuleV2(ctx, moduleV2, origin, conf); err != nil {
		return false, err
	}

	return true, nil
}

// createModuleV2 creates a v1alpha2 Module the cluster does not carry yet.
func (s *syncer) createModuleV2(ctx context.Context, name string, origin Origin, conf *v1alpha1.ModuleConfig) (*v1alpha2.Module, error) {
	moduleV2 := &v1alpha2.Module{ObjectMeta: metav1.ObjectMeta{Name: name}}
	fillModuleV2(moduleV2, origin, conf)

	if err := s.writer.Create(ctx, moduleV2); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("create module '%s': %w", name, err)
		}

		// something created the module between the list and this call, so converge it here: the
		// sync runs once, and an object left as the racing writer made it stays that way
		moduleV2 = new(v1alpha2.Module)
		if err := s.reader.Get(ctx, client.ObjectKey{Name: name}, moduleV2); err != nil {
			return nil, fmt.Errorf("get module '%s': %w", name, err)
		}

		if err := s.patchModuleV2(ctx, moduleV2, origin, conf); err != nil {
			return nil, err
		}
	}

	return moduleV2, nil
}

// patchModuleV2 writes origin, annotations and settings into the v1alpha2
// Module in one patch, and nothing when none drifted.
func (s *syncer) patchModuleV2(ctx context.Context, moduleV2 *v1alpha2.Module, origin Origin, conf *v1alpha1.ModuleConfig) error {
	patch := client.MergeFrom(moduleV2.DeepCopy())

	fillModuleV2(moduleV2, origin, conf)

	data, err := patch.Data(moduleV2)
	if err != nil {
		return fmt.Errorf("build patch for the module '%s': %w", moduleV2.Name, err)
	}

	if string(data) == "{}" {
		return nil
	}

	if err := s.writer.Patch(ctx, moduleV2, client.RawPatch(patch.Type(), data)); err != nil {
		return fmt.Errorf("patch module '%s': %w", moduleV2.Name, err)
	}

	s.logger.Debug("module synced", slog.String("name", moduleV2.Name), slog.String("version", origin.PackageVersion))

	return nil
}
