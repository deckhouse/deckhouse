// Copyright 2025 Flant JSC
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

package loader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/dto"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules/global"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/tools/verity"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	loaderTracer = "package-loader"

	digestsFile = "images_digests.json"

	globalPath = "global-hooks"
)

var (
	// ErrPackageNotFound is returned when the requested package directory doesn't exist
	ErrPackageNotFound = errors.New("package not found")
)

// LoadAppConf loads an application package from the filesystem based on the instance specification.
// It performs the following steps:
//  1. Validates package directory exists
//  2. Loads package definition (package.yaml)
//  3. Loads values (static values.yaml and OpenAPI schemas)
//  4. Discovers and loads hooks
//  5. Creates and returns an Application config
//
// Returns ErrPackageNotFound if package directory doesn't exist.
func LoadAppConf(ctx context.Context, appDir string, logger *log.Logger) (*apps.Config, error) {
	ctx, span := otel.Tracer(loaderTracer).Start(ctx, "LoadAppConf")
	defer span.End()

	span.SetAttributes(attribute.String("path", appDir))

	logger = logger.With(slog.String("path", appDir))

	logger.Debug("load application from directory")

	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		span.SetStatus(codes.Error, ErrPackageNotFound.Error())
		return nil, ErrPackageNotFound
	}

	// Load package definition (package.yaml)
	def, err := loadPackageDefinition(ctx, appDir)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load package from '%s': %w", appDir, err)
	}

	// Load values from values.yaml and openapi schemas
	static, config, values, err := loadValues(def.Name, appDir)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load values: %w", err)
	}

	appName := filepath.Base(appDir)

	splits := strings.SplitN(appName, ".", 2)
	if len(splits) != 2 {
		span.SetStatus(codes.Error, "invalid name")
		return nil, fmt.Errorf("invalid package name '%s'", appName)
	}

	// Discover and load hooks (shell and batch)
	hooks, err := loadAppHooks(ctx, splits[0], splits[1], appDir, logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load hooks: %w", err)
	}

	appDef, err := def.ToApplication()
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("convert app definition: %w", err)
	}

	digests, err := loadDigests(appDir)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load digests: %w", err)
	}

	return &apps.Config{
		Path:       appDir,
		Definition: appDef,

		Digests: digests,

		StaticValues: static,
		ConfigSchema: config,
		ValuesSchema: values,

		Hooks: hooks.hooks,

		SettingsCheck: hooks.settingsCheck,
	}, nil
}

// LoadModuleConf loads a module package from the filesystem based on the instance specification.
// It performs the following steps:
//  1. Validates package directory exists
//  2. Loads package definition (module.yaml)
//  3. Loads values (static values.yaml and OpenAPI schemas)
//  4. Discovers and loads hooks
//  5. Creates and returns a Module config
//
// Returns ErrPackageNotFound if package directory doesn't exist.
func LoadModuleConf(ctx context.Context, moduleDir string, logger *log.Logger) (*modules.Config, error) {
	ctx, span := otel.Tracer(loaderTracer).Start(ctx, "LoadModuleConf")
	defer span.End()

	span.SetAttributes(attribute.String("path", moduleDir))

	logger = logger.With(slog.String("path", moduleDir))

	logger.Debug("load module from directory", slog.String("path", moduleDir))

	if _, err := os.Stat(moduleDir); os.IsNotExist(err) {
		span.SetStatus(codes.Error, ErrPackageNotFound.Error())
		return nil, ErrPackageNotFound
	}

	// Load package definition (package.yaml/module.yaml)
	def, err := loadPackageDefinition(ctx, moduleDir)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load package from '%s': %w", moduleDir, err)
	}

	// Load values from values.yaml and openapi schemas
	static, config, values, err := loadValues(def.Name, moduleDir)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load values: %w", err)
	}

	// Discover and load hooks (shell and batch)
	hooks, err := loadModuleHooks(ctx, def.Name, moduleDir, logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load hooks: %w", err)
	}

	moduleDef, err := def.ToModule()
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("convert module definition: %w", err)
	}

	digests, err := loadDigests(moduleDir)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load digests: %w", err)
	}

	return &modules.Config{
		Path:       moduleDir,
		Definition: moduleDef,

		Digests: digests,

		StaticValues: static,
		ConfigSchema: config,
		ValuesSchema: values,

		Hooks: hooks.hooks,

		SettingsCheck: hooks.settingsCheck,
	}, nil
}

// LoadGlobalConf loads the global module configuration from the filesystem.
// It performs the following steps:
//  1. Validates the global module directory exists
//  2. Loads values (static values.yaml and OpenAPI schemas)
//  3. Discovers and loads global hooks
//  4. Creates and returns a global Config
//
// Returns ErrPackageNotFound if the global module directory doesn't exist.
func LoadGlobalConf(ctx context.Context, logger *log.Logger) (*global.Config, error) {
	ctx, span := otel.Tracer(loaderTracer).Start(ctx, "LoadGlobalConf")
	defer span.End()

	span.SetAttributes(attribute.String("path", globalPath))

	logger = logger.With(slog.String("path", globalPath))

	logger.Debug("load global module from directory", slog.String("path", globalPath))

	if _, err := os.Stat(globalPath); os.IsNotExist(err) {
		span.SetStatus(codes.Error, ErrPackageNotFound.Error())
		return nil, ErrPackageNotFound
	}

	// Load values from values.yaml and openapi schemas
	static, config, values, err := loadValues("global", globalPath)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load values: %w", err)
	}

	// Load hooks from registry
	hooks, err := loadGlobalHooks(ctx, logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load hooks: %w", err)
	}

	return &global.Config{
		Path: globalPath,

		StaticValues: static,
		ConfigSchema: config,
		ValuesSchema: values,

		Hooks: hooks,
	}, nil
}

// loadPackageDefinition reads and parses the package.yaml file from the package directory.
// It validates YAML structure but doesn't validate content.
//
// Returns the parsed Definition or an error if reading or parsing fails.
func loadPackageDefinition(ctx context.Context, packageDir string) (*dto.Definition, error) {
	definitionPath := filepath.Join(packageDir, dto.DefinitionFile)

	content, err := os.ReadFile(definitionPath)
	if err == nil {
		def := new(dto.Definition)
		if err = yaml.Unmarshal(content, def); err != nil {
			return nil, fmt.Errorf("unmarshal file '%s': %w", definitionPath, err)
		}

		return def, nil
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read file '%s': %w", definitionPath, err)
	}

	def, err := loadModuleDefinition(packageDir)
	if err != nil {
		return nil, fmt.Errorf("load module definition: %w", err)
	}

	// TODO(ipaqsa): its better to have version injected into package.yaml, but we can retrieve by fs
	version, err := getModuleVersion(ctx, packageDir)
	if err != nil {
		return nil, fmt.Errorf("load module version: %w", err)
	}

	return &dto.Definition{
		Name:    def.Name,
		Type:    "Module",
		Version: version,
		Stage:   def.Stage,
		Descriptions: dto.Descriptions{
			Ru: def.Descriptions.Ru,
			En: def.Descriptions.En,
		},
		Requirements: dto.Requirements{
			Kubernetes: def.Requirements.Kubernetes,
			Deckhouse:  def.Requirements.Deckhouse,
		},
		DisableOptions: dto.DisableOptions{
			Confirmation: def.DisableOptions.Confirmation,
			Message:      def.DisableOptions.Message,
		},
		Module: dto.DefinitionModule{
			Weight:   int(def.Weight),
			Critical: def.Critical,
		},
	}, nil
}

// loadModuleDefinition reads and parses the module.yaml file from the package directory.
// It validates YAML structure but doesn't validate content.
//
// Returns the parsed Definition or an error if reading or parsing fails.
// TODO(ipaqsa): get rid of it when all modules migrated to package.yaml
func loadModuleDefinition(packageDir string) (*moduletypes.Definition, error) {
	definitionPath := filepath.Join(packageDir, moduletypes.DefinitionFile)

	content, err := os.ReadFile(definitionPath)
	if err != nil {
		return nil, fmt.Errorf("read definition file '%s': %w", definitionPath, err)
	}

	def := new(moduletypes.Definition)
	if err = yaml.Unmarshal(content, def); err != nil {
		return nil, fmt.Errorf("unmarshal file '%s': %w", definitionPath, err)
	}

	return def, nil
}

// loadDigests reads and parses the images_digests.json file from package directory.
// The file contains package images hashes
func loadDigests(packageDir string) (map[string]string, error) {
	path := filepath.Join(packageDir, digestsFile)

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read file '%s': %w", path, err)
	}

	digests := make(map[string]string)
	if err = json.Unmarshal(content, &digests); err != nil {
		return nil, fmt.Errorf("unmarshal file '%s': %w", path, err)
	}

	return digests, nil
}

// getModuleVersion returns the version of the package at moduleDir.
// With dm-verity, the version is extracted from the device status.
// Without dm-verity, the version is derived from the symlink target directory name.
func getModuleVersion(ctx context.Context, moduleDir string) (string, error) {
	if verity.IsSupported() {
		return verity.GetVersionByDevice(ctx, filepath.Base(moduleDir))
	}

	// resolve symlink to get the versioned directory name
	target, err := os.Readlink(moduleDir)
	if err != nil {
		return "", fmt.Errorf("readlink '%s': %w", moduleDir, err)
	}

	return filepath.Base(target), nil
}
