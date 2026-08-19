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

// This file writes the desired state into the cluster: one snapshot of the
// v1alpha2 Module resources, then create, merge-patch or delete per module. The field
// mapping itself lives in fill.go.

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

// writeModulesV2 brings every v1alpha2 Module in line with its origin and its config, and
// returns the survivors carrying what was written.
func (s *Syncer) writeModulesV2(ctx context.Context, origins map[string]Origin, configs map[string]*v1alpha1.ModuleConfig) ([]v1alpha2.Module, error) {
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

// writeExistingModuleV2 patches the module to its origin and config, or deletes it when it
// is orphaned. It reports whether the module survived.
func (s *Syncer) writeExistingModuleV2(ctx context.Context, moduleV2 *v1alpha2.Module, origin Origin, conf *v1alpha1.ModuleConfig) (bool, error) {
	// the module is orphaned when no source claims it and, on top of that,
	// no writer ever put a package version here - or only the image supplied
	// it and no longer ships it
	neverFilled := moduleV2.Spec.PackageVersion == ""
	imageOnly := moduleV2.IsEmbedded() && moduleV2.Spec.PackageRepositoryName == embeddedRepositoryName

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
func (s *Syncer) createModuleV2(ctx context.Context, name string, origin Origin, conf *v1alpha1.ModuleConfig) (*v1alpha2.Module, error) {
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

// patchModuleV2 writes origin, annotations and settings into the v1alpha2 Module in one
// patch, and nothing when none drifted.
func (s *Syncer) patchModuleV2(ctx context.Context, moduleV2 *v1alpha2.Module, origin Origin, conf *v1alpha1.ModuleConfig) error {
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
