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

package controller

import (
	"context"
	"fmt"
	"log/slog"

	"sigs.k8s.io/controller-runtime/pkg/client"

	pkgmodules "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	pkgruntime "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
)

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

// cleanupPackages hands the runtime every package the cluster still claims, so it drops the rest.
// A terminating instance is left out, as in loadModules: the runtime forgets its teardown across a
// restart and never loads a terminating object, so the remover answers "nothing left to tear down"
// and this pass is the last owner of its release.
func (c *Controller) cleanupPackages(ctx context.Context, modules []v1alpha2.Module) error {
	// this list decides what is deleted, so a lagging watch would read as an application gone
	applications := new(v1alpha1.ApplicationList)
	if err := c.ctrl.GetAPIReader().List(ctx, applications); err != nil {
		return fmt.Errorf("list applications: %w", err)
	}

	preserveApps := make([]pkgruntime.PreserveApplication, 0, len(applications.Items))
	for i := range applications.Items {
		application := &applications.Items[i]

		if !application.DeletionTimestamp.IsZero() {
			continue
		}

		preserveApps = append(preserveApps, pkgruntime.PreserveApplication{
			Namespace:   application.Namespace,
			Name:        application.Name,
			PackageName: application.Spec.PackageName,
			Repository:  application.Spec.PackageRepositoryName,
			Version:     application.Spec.PackageVersion,
		})
	}

	preserveModules := make([]pkgruntime.PreserveModule, 0, len(modules))
	for i := range modules {
		module := &modules[i]

		if !module.DeletionTimestamp.IsZero() {
			continue
		}

		preserveModules = append(preserveModules, pkgruntime.PreserveModule{
			Name:       module.Name,
			Repository: module.Spec.PackageRepositoryName,
			Version:    module.Spec.PackageVersion,
			Embedded:   module.IsEmbedded(),
		})
	}

	return c.manager.CleanupV2(ctx, preserveApps, preserveModules)
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
