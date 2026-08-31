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

// This file resolves origins from ModulePullOverride resources, which the
// package system replaces. An override pins a module to a mutable image tag
// and carries no repository, so the repository is derived from the synced
// resources around the module. The file dies together with the
// ModulePullOverride deprecation.

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
)

// originsFromModulePullOverrides pins every module a ready pull override
// names to the tag it carries. The repository comes from the module's other
// synced resources; a module with no trace of one gives no repository to pull
// from, and claiming it would only hide the release that does know one.
func (s *syncer) originsFromModulePullOverrides(ctx context.Context, fromReleases map[string]Origin, configs map[string]*v1alpha1.ModuleConfig) (map[string]Origin, error) {
	overrides := new(v1alpha2.ModulePullOverrideList)
	if err := s.reader.List(ctx, overrides); err != nil {
		return nil, fmt.Errorf("list module overrides: %w", err)
	}

	origins := make(map[string]Origin, len(overrides.Items))

	for _, mpo := range overrides.Items {
		if !mpo.DeletionTimestamp.IsZero() || mpo.Status.Message != v1alpha1.ModulePullOverrideMessageReady {
			continue
		}

		moduleV2 := new(v1alpha2.Module)
		if err := s.reader.Get(ctx, client.ObjectKey{Name: mpo.Name}, moduleV2); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("get module '%s': %w", mpo.Name, err)
			}

			moduleV2 = nil
		}

		repository, err := s.devModuleRepository(ctx, mpo.Name, fromReleases[mpo.Name], configs[mpo.Name], moduleV2)
		if err != nil {
			return nil, err
		}

		if repository == "" {
			s.logger.Info("no synced resource names the module's repository, skip its pull override",
				slog.String("name", mpo.Name))

			continue
		}

		origins[mpo.Name] = Origin{RepositoryName: repository, PackageVersion: mpo.Spec.ImageTag, Dev: true}
	}

	return origins, nil
}

// devModuleRepository resolves which repository serves a module a pull
// override pins. The override does not carry one, so the answer comes from
// the synced resources around the module, first found wins:
//
//   - the module's deployed release origin;
//   - the ModuleConfig naming a source;
//   - the repository already on the Module spec;
//   - the package versions of the module, when they all name one repository;
//   - the single ModuleSource offering the module.
//
// An empty result means no resource knows the repository; the caller skips
// the override.
func (s *syncer) devModuleRepository(ctx context.Context, moduleName string, fromRelease Origin, conf *v1alpha1.ModuleConfig, moduleV2 *v1alpha2.Module) (string, error) {
	if fromRelease.RepositoryName != "" {
		return fromRelease.RepositoryName, nil
	}

	if conf != nil && conf.Spec.Source != "" {
		return repositoryNameForSource(conf.Spec.Source), nil
	}

	// the embedded sentinel names no repository to pull a dev tag from
	if moduleV2 != nil && moduleV2.Spec.PackageRepositoryName != "" &&
		moduleV2.Spec.PackageRepositoryName != repositoryNameEmbedded {
		return repositoryNameForSource(moduleV2.Spec.PackageRepositoryName), nil
	}

	repository, err := s.repositoryFromPackageVersions(ctx, moduleName)
	if err != nil || repository != "" {
		return repository, err
	}

	return s.repositoryFromModuleSources(ctx, moduleName)
}

// repositoryFromPackageVersions answers with the repository the module's
// package versions name, when they all name the same one. Versions of several
// repositories name none: picking one would be a guess.
func (s *syncer) repositoryFromPackageVersions(ctx context.Context, moduleName string) (string, error) {
	versions := new(v1alpha1.ModulePackageVersionList)
	selector := client.MatchingLabels{v1alpha1.ModulePackageVersionLabelPackage: moduleName}
	if err := s.reader.List(ctx, versions, selector); err != nil {
		return "", fmt.Errorf("list package versions of the module '%s': %w", moduleName, err)
	}

	repository := ""

	for i := range versions.Items {
		name := versions.Items[i].Spec.PackageRepositoryName
		if name == "" || name == repositoryNameEmbedded {
			continue
		}

		if repository != "" && repository != name {
			s.logger.Info("package versions of the module name several repositories",
				slog.String("name", moduleName))

			return "", nil
		}

		repository = name
	}

	return repository, nil
}

// repositoryFromModuleSources answers with the repository of the single
// module source offering the module. Several offers name none: picking one
// would be a guess.
func (s *syncer) repositoryFromModuleSources(ctx context.Context, moduleName string) (string, error) {
	sources := new(v1alpha1.ModuleSourceList)
	if err := s.reader.List(ctx, sources); err != nil {
		return "", fmt.Errorf("list module sources: %w", err)
	}

	sourceName := ""

	for i := range sources.Items {
		source := &sources.Items[i]

		offered := slices.ContainsFunc(source.Status.AvailableModules, func(module v1alpha1.AvailableModule) bool {
			return module.Name == moduleName
		})
		if !offered {
			continue
		}

		if sourceName != "" {
			s.logger.Info("several module sources offer the module",
				slog.String("name", moduleName))

			return "", nil
		}

		sourceName = source.Name
	}

	return repositoryNameForSource(sourceName), nil
}
