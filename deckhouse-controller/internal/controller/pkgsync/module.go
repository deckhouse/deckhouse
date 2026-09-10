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

package pkgsync

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// syncModules writes a Module for every embedded module.
// Every embedded package of one Deckhouse build carries the same version, the
// one app.EmbeddedPackageVersion reduces the Deckhouse version to.
func (s *syncer) syncModules(ctx context.Context) error {
	embeddedPackageVersion := app.EmbeddedPackageVersion(s.deckhouseVersion)

	moduleNames, err := s.embeddedModuleNames()
	if err != nil {
		return err
	}

	for _, moduleName := range moduleNames {
		if err := s.ensureEmbeddedModule(ctx, moduleName, embeddedPackageVersion); err != nil {
			return err
		}
	}

	return nil
}

// embeddedModuleNames reads the names of the modules from the embedded modules dir.
// A dir with no readable definition is skipped with a warning, the same way its
// package version is.
func (s *syncer) embeddedModuleNames() ([]string, error) {
	entries, err := os.ReadDir(s.embeddedModulesDir)
	if err != nil {
		return nil, fmt.Errorf("read embedded modules dir: %w", err)
	}

	moduleNames := make([]string, 0, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() || slices.Contains(app.DummyModules, entry.Name()) {
			continue
		}

		moduleDir := filepath.Join(s.embeddedModulesDir, entry.Name())

		definition, err := loader.LoadEmbeddedDefinition(moduleDir)
		if err != nil {
			s.logger.Warn("module dir holds no readable definition, skip its module",
				slog.String("dir", moduleDir), log.Err(err))

			continue
		}

		moduleNames = append(moduleNames, definition.Name)
	}

	return moduleNames, nil
}

// ensureEmbeddedModule points the module at its embedded package:
// - the object is created when the cluster carries none
// - the embedded repository name is reserved, no PackageRepository serves it
// - only the fields below are written, the module has other writers
// - a patch with no drift is not sent
func (s *syncer) ensureEmbeddedModule(ctx context.Context, moduleName, packageVersion string) error {
	module := new(v1alpha2.Module)

	if err := s.reader.Get(ctx, client.ObjectKey{Name: moduleName}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get the '%s' module: %w", moduleName, err)
		}

		return s.createEmbeddedModule(ctx, moduleName, packageVersion)
	}

	patch := client.MergeFrom(module.DeepCopy())

	applyEmbeddedModule(module, packageVersion)

	patchData, err := patch.Data(module)
	if err != nil {
		return fmt.Errorf("build the patch for the '%s' module: %w", moduleName, err)
	}

	if string(patchData) == "{}" {
		return nil
	}

	if err = s.writer.Patch(ctx, module, client.RawPatch(patch.Type(), patchData)); err != nil {
		return fmt.Errorf("patch the '%s' module: %w", moduleName, err)
	}

	s.logger.Debug("module synced", slog.String("name", moduleName), slog.String("version", packageVersion))

	return nil
}

// createEmbeddedModule writes a module the cluster does not carry yet.
// Rare: the old module stack creates an object for every module it knows.
func (s *syncer) createEmbeddedModule(ctx context.Context, moduleName, packageVersion string) error {
	module := &v1alpha2.Module{ObjectMeta: metav1.ObjectMeta{Name: moduleName}}

	applyEmbeddedModule(module, packageVersion)

	if err := s.writer.Create(ctx, module); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create the '%s' module: %w", moduleName, err)
	}

	s.logger.Debug("module created", slog.String("name", moduleName), slog.String("version", packageVersion))

	return nil
}

// applyEmbeddedModule points the module spec at its embedded package and marks
// the module as embedded.
func applyEmbeddedModule(module *v1alpha2.Module, packageVersion string) {
	module.Spec.PackageRepositoryName = repositoryNameEmbedded
	module.Spec.PackageVersion = packageVersion

	if module.Annotations == nil {
		module.Annotations = make(map[string]string, 1)
	}

	module.Annotations[v1alpha2.ModuleAnnotationEmbedded] = "true"
}
