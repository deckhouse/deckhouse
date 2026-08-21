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

package modulesync

// This file resolves origins from ModulePullOverride resources, which the
// package system replaces. It dies together with their deprecation; the rest
// of the package touches it only through mergeOrigins.

import (
	"context"
	"fmt"
	"log/slog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
)

// originsFromModulePullOverrides pins every module a ready pull override names to the tag it carries.
func (s *Syncer) originsFromModulePullOverrides(ctx context.Context) (map[string]Origin, error) {
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
