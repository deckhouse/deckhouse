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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

// ensureModulePackageExists makes sure the catalog names the embedded module:
// an empty ModulePackage is created once and never touched again. The repository
// scan enriches the same object with owners and available repositories once a
// repository offers the package; the embedded entry itself has no owner, so it
// outlives every repository.
func (s *syncer) ensureModulePackageExists(ctx context.Context, name string) error {
	err := s.reader.Get(ctx, client.ObjectKey{Name: name}, new(v1alpha1.ModulePackage))
	if err == nil {
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get module package '%s': %w", name, err)
	}

	pkg := &v1alpha1.ModulePackage{
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

		// another writer created it between the read and this call; theirs wins
		return nil
	}

	s.logger.Debug("module package created", slog.String("name", name))

	return nil
}
