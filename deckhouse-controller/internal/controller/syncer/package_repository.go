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

// excludedModuleSources lists the platform-owned sources. The platform ships
// their repositories itself with live registry credentials, like the
// "deckhouse" repository from the deckhouse module templates; a snapshot of
// the source credentials would go stale.
var excludedModuleSources = []string{moduleSourceNameDeckhouse, moduleSourceNameFlant}

// syncPackageRepositories ensures a PackageRepository for every module source
// except the excluded ones, so the repositories are in place before the
// version sync leaves its draft stubs. A source being deleted gets none.
func (s *Syncer) syncPackageRepositories(ctx context.Context) error {
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

// ensurePackageRepository makes sure the repository serving the source
// registry exists. An existing repository is left as is, so user edits
// survive a restart. No scan interval is set: the package-repository
// controller persists its default into the spec itself.
func (s *Syncer) ensurePackageRepository(ctx context.Context, source *v1alpha1.ModuleSource) error {
	name := repositoryNameForSource(source.Name)

	err := s.reader.Get(ctx, client.ObjectKey{Name: name}, new(v1alpha1.PackageRepository))
	if err == nil {
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get package repository '%s': %w", name, err)
	}

	repo := &v1alpha1.PackageRepository{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.PackageRepositoryGVK.GroupVersion().String(),
			Kind:       v1alpha1.PackageRepositoryKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"heritage": "deckhouse"},
		},
		Spec: v1alpha1.PackageRepositorySpec{
			Registry: v1alpha1.PackageRepositorySpecRegistry{
				Scheme:    source.Spec.Registry.Scheme,
				Repo:      source.Spec.Registry.Repo,
				DockerCFG: source.Spec.Registry.DockerCFG,
				CA:        source.Spec.Registry.CA,
			},
		},
	}

	if err := s.writer.Create(ctx, repo); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create package repository '%s': %w", name, err)
		}

		// another writer created it between the read and this call; theirs wins
		return nil
	}

	s.logger.Debug("package repository created", slog.String("name", name))

	return nil
}
