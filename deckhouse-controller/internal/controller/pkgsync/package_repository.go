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
	"fmt"
	"log/slog"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

// excludedModuleSources lists the platform-owned sources. The platform ships
// their repositories itself with live registry credentials, like the
// "deckhouse" repository from the deckhouse module templates; a snapshot of
// the source credentials would go stale.
var excludedModuleSources = []string{v1alpha1.ModuleSourceNameDeckhouse, moduleSourceNameFlant}

// syncPackageRepositories ensures a PackageRepository for every module source
// except the excluded ones, so the repositories are in place before the
// version sync leaves its draft stubs. A source being deleted gets none.
func (s *syncer) syncPackageRepositories(ctx context.Context) error {
	sources := new(v1alpha1.ModuleSourceList)
	if err := s.reader.List(ctx, sources); err != nil {
		return fmt.Errorf("list module sources: %w", err)
	}

	for idx := range sources.Items {
		source := &sources.Items[idx]
		if slices.Contains(excludedModuleSources, source.Name) || !source.DeletionTimestamp.IsZero() {
			continue
		}

		if err := s.ensurePackageRepository(ctx, source); err != nil {
			return err
		}
	}

	return nil
}

// ensurePackageRepository converges the repository serving the source registry
// to the source: created if missing, the registry settings brought to what the
// source holds. The module source stays the place where users manage the
// credentials of a source-backed repository while the old stack lives. The
// fields the source does not carry (scan interval, login, password) are never
// touched, so user edits to them survive a restart.
func (s *syncer) ensurePackageRepository(ctx context.Context, source *v1alpha1.ModuleSource) error {
	name := v1alpha1.PackageRepositoryNameForModuleSource(source.Name)
	desired := registryFromSource(source)

	repo := new(v1alpha1.PackageRepository)
	err := s.reader.Get(ctx, client.ObjectKey{Name: name}, repo)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get package repository '%s': %w", name, err)
	}

	if apierrors.IsNotFound(err) {
		repo = &v1alpha1.PackageRepository{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.PackageRepositoryGVK.GroupVersion().String(),
				Kind:       v1alpha1.PackageRepositoryKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{"heritage": "deckhouse"},
			},
			Spec: v1alpha1.PackageRepositorySpec{Registry: desired},
		}

		if err := s.writer.Create(ctx, repo); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("create package repository '%s': %w", name, err)
			}

			// another writer created it between the read and this call; the
			// next start converges it
			return nil
		}

		s.logger.Debug("package repository created", slog.String("name", name))

		return nil
	}

	current := repo.Spec.Registry
	current.Login, current.Password = "", ""
	if current == desired {
		return nil
	}

	original := repo.DeepCopy()
	desired.Login = repo.Spec.Registry.Login
	desired.Password = repo.Spec.Registry.Password
	repo.Spec.Registry = desired

	if err := s.writer.Patch(ctx, repo, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch package repository '%s': %w", name, err)
	}

	s.logger.Debug("package repository registry refreshed from the module source", slog.String("name", name))

	return nil
}

// registryFromSource maps the source registry block onto the repository shape.
// Login and password have no source counterpart and stay zero.
func registryFromSource(source *v1alpha1.ModuleSource) v1alpha1.PackageRepositorySpecRegistry {
	return v1alpha1.PackageRepositorySpecRegistry{
		Scheme:    source.Spec.Registry.Scheme,
		Repo:      source.Spec.Registry.Repo,
		DockerCFG: source.Spec.Registry.DockerCFG,
		CA:        source.Spec.Registry.CA,
	}
}
