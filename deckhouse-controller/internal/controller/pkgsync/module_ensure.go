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

// This file is for the old module stack controllers: on their events they
// mirror what changed into the v1alpha2 Module, one module at a time, with
// the same field mapping the startup sync uses. All reads go through reader,
// and callers pass a direct (uncached) one, so a mirror sees its own prior
// writes. The file dies together with those controllers.

import (
	"context"
	"fmt"
	"log/slog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// OriginFromPullOverride is the origin of a module a ready ModulePullOverride
// pins. The override carries no repository; EnsureModule derives it from the
// synced resources around the module.
func OriginFromPullOverride(imageTag string) Origin {
	return Origin{PackageVersion: imageTag, Dev: true}
}

// OriginFromDeployedRelease is the origin of a module a deployed ModuleRelease
// serves. A module source name maps to the name of the PackageRepository
// serving the same registry path.
func OriginFromDeployedRelease(sourceName, version string) Origin {
	return Origin{RepositoryName: v1alpha1.PackageRepositoryNameForModuleSource(sourceName), PackageVersion: version}
}

// EnsureModule converges one v1alpha2 Module with its origin. When the write
// is the first to put a version on the module, it also fills the ModuleConfig
// fields: they were gated on the version until now, and no config event will
// replay them.
func EnsureModule(ctx context.Context, reader client.Reader, writer client.Client, moduleName string, origin Origin, logger *log.Logger) error {
	s := &syncer{reader: reader, writer: writer, logger: logger}

	moduleV2 := new(v1alpha2.Module)
	if err := s.reader.Get(ctx, client.ObjectKey{Name: moduleName}, moduleV2); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get module '%s': %w", moduleName, err)
		}

		moduleV2 = nil
	}

	// a pull override names no repository of its own
	if origin.Dev && origin.RepositoryName == "" {
		conf, err := s.moduleConfig(ctx, moduleName)
		if err != nil {
			return err
		}

		repository, err := s.devModuleRepository(ctx, moduleName, Origin{}, conf, moduleV2)
		if err != nil {
			return err
		}

		if repository == "" {
			s.logger.Info("no synced resource names the module's repository, skip its pull override",
				slog.String("name", moduleName))

			return nil
		}

		origin.RepositoryName = repository
	}

	if moduleV2 == nil {
		conf, err := s.moduleConfig(ctx, moduleName)
		if err != nil {
			return err
		}

		if _, err := s.createModuleV2(ctx, moduleName, origin, conf); err != nil {
			return err
		}

		return nil
	}

	// seeding: the version arrives on a module that had none
	var conf *v1alpha1.ModuleConfig
	if origin.Known() && moduleV2.Spec.PackageVersion == "" {
		var err error
		if conf, err = s.moduleConfig(ctx, moduleName); err != nil {
			return err
		}
	}

	return s.patchModuleV2(ctx, moduleV2, origin, conf)
}

// EnsureModuleConfig mirrors the config fields onto the v1alpha2 Module. A
// module that does not exist or carries no version yet is skipped: the
// version mirror fills the config fields when it seeds the module.
func EnsureModuleConfig(ctx context.Context, reader client.Reader, writer client.Client, conf *v1alpha1.ModuleConfig, logger *log.Logger) error {
	s := &syncer{reader: reader, writer: writer, logger: logger}

	moduleV2 := new(v1alpha2.Module)
	if err := s.reader.Get(ctx, client.ObjectKey{Name: conf.Name}, moduleV2); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get module '%s': %w", conf.Name, err)
		}

		s.logger.Debug("no module for the config, skip mirroring", slog.String("name", conf.Name))

		return nil
	}

	if moduleV2.Spec.PackageVersion == "" {
		s.logger.Debug("module has no package version yet, skip mirroring the config",
			slog.String("name", conf.Name))

		return nil
	}

	return s.patchModuleV2(ctx, moduleV2, Origin{}, conf)
}

// DeleteModuleConfig clears the config fields of the v1alpha2 Module when its
// ModuleConfig is deleted, so the mirror does not keep serving settings the
// cluster no longer holds. The origin fields stay untouched: they report where
// the package came from, not what the config asked for.
func DeleteModuleConfig(ctx context.Context, reader client.Reader, writer client.Client, moduleName string, logger *log.Logger) error {
	s := &syncer{reader: reader, writer: writer, logger: logger}

	moduleV2 := new(v1alpha2.Module)
	if err := s.reader.Get(ctx, client.ObjectKey{Name: moduleName}, moduleV2); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get module '%s': %w", moduleName, err)
		}

		return nil
	}

	patch := client.MergeFrom(moduleV2.DeepCopy())

	moduleV2.Spec.Settings = nil
	moduleV2.Spec.SettingsVersion = 0
	moduleV2.Spec.Maintenance = ""
	moduleV2.Spec.Enabled = nil
	moduleV2.Spec.UpdatePolicy = ""

	data, err := patch.Data(moduleV2)
	if err != nil {
		return fmt.Errorf("build patch for the module '%s': %w", moduleName, err)
	}

	if string(data) == "{}" {
		return nil
	}

	if err := s.writer.Patch(ctx, moduleV2, client.RawPatch(patch.Type(), data)); err != nil {
		return fmt.Errorf("patch module '%s': %w", moduleName, err)
	}

	s.logger.Debug("module config fields cleared", slog.String("name", moduleName))

	return nil
}

// moduleConfig reads the module's live config; a missing or deleting config is nil.
func (s *syncer) moduleConfig(ctx context.Context, name string) (*v1alpha1.ModuleConfig, error) {
	conf := new(v1alpha1.ModuleConfig)
	if err := s.reader.Get(ctx, client.ObjectKey{Name: name}, conf); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("get module config '%s': %w", name, err)
		}

		return nil, nil
	}

	if !conf.DeletionTimestamp.IsZero() {
		return nil, nil
	}

	return conf, nil
}
