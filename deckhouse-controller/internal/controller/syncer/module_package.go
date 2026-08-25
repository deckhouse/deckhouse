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

package syncer

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

// syncModulePackages ensures a ModulePackage for every module the old module
// sources offer, so the catalog knows where each package is available before
// any repository scan ran.
func (s *Syncer) syncModulePackages(ctx context.Context) error {
	modules := new(v1alpha1.ModuleList)
	if err := s.reader.List(ctx, modules); err != nil {
		return fmt.Errorf("list modules: %w", err)
	}

	for idx := range modules.Items {
		module := &modules.Items[idx]

		// an embedded module comes from no source; the scan never creates a
		// package for it either
		if module.IsEmbedded() || len(module.Properties.AvailableSources) == 0 {
			continue
		}

		if err := s.ensureModulePackage(ctx, module.Name, repositoriesForSources(module.Properties.AvailableSources)); err != nil {
			return err
		}
	}

	return nil
}

// repositoriesForSources maps module source names onto repository names,
// dropping the duplicates the mapping may fold together.
func repositoriesForSources(sources []string) []string {
	repositories := make([]string, 0, len(sources))

	for _, source := range sources {
		repository := repositoryNameForSource(source)
		if !slices.Contains(repositories, repository) {
			repositories = append(repositories, repository)
		}
	}

	return repositories
}

// ensureModulePackage makes sure the package exists with the given repositories
// in its status. An existing package is left untouched unless its repository
// list is empty, which a create interrupted before the status patch leaves
// behind; such a package is completed in place. No owner is set: the
// repositories the status names may not exist yet, and the scan adopts the
// package once one appears.
func (s *Syncer) ensureModulePackage(ctx context.Context, name string, repositories []string) error {
	pkg := new(v1alpha1.ModulePackage)

	err := s.reader.Get(ctx, client.ObjectKey{Name: name}, pkg)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get module package '%s': %w", name, err)
	}

	if apierrors.IsNotFound(err) {
		pkg = &v1alpha1.ModulePackage{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.ModulePackageGVK.GroupVersion().String(),
				Kind:       v1alpha1.ModulePackageKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{"heritage": "deckhouse"},
			},
		}

		if err := s.writer.Create(ctx, pkg); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("create module package '%s': %w", name, err)
			}

			// another writer created it between the read and this call;
			// converge on whatever exists now
			if err := s.reader.Get(ctx, client.ObjectKey{Name: name}, pkg); err != nil {
				return fmt.Errorf("get module package '%s': %w", name, err)
			}
		}
	}

	if len(pkg.Status.AvailableRepositories) > 0 {
		return nil
	}

	original := pkg.DeepCopy()
	pkg.Status.AvailableRepositories = repositories

	if err := s.writer.Status().Patch(ctx, pkg, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch module package status '%s': %w", name, err)
	}

	s.logger.Debug("module package created", slog.String("name", name))

	return nil
}
