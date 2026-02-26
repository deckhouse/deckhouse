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
	// loaderTracer is the OpenTelemetry tracer name for package loading operations.
	loaderTracer = "package-loader"

	// digestsFile is the JSON file mapping image names to their content-addressable digests.
	digestsFile = "images_digests.json"

	// globalPath is the relative directory containing global hook definitions and values.
	// LoadGlobalConf expects this path to exist relative to the process working directory.
	globalPath = "global-hooks"
)

var (
	// ErrPackageNotFound is returned when the requested package directory doesn't exist
	ErrPackageNotFound = errors.New("package not found")
)

// LoadAppConf loads an application package from the given directory on the filesystem.
// The directory name must follow the "namespace.name" convention (e.g., "default.my-app").
//
// Steps:
//  1. Validates the package directory exists
//  2. Loads the package definition (package.yaml)
//  3. Loads values (static values.yaml and OpenAPI schemas)
//  4. Extracts namespace and name from the directory basename
//  5. Discovers and loads batch hooks
//  6. Loads image digests (images_digests.json)
//
// Returns ErrPackageNotFound if the directory doesn't exist.
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

	def, err := loadPackageDefinition(ctx, appDir)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load package from '%s': %w", appDir, err)
	}

	static, config, values, err := loadValues(def.Name, appDir)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load values: %w", err)
	}

	// Extract namespace and name from directory basename (e.g., "default.my-app")
	appName := filepath.Base(appDir)

	splits := strings.SplitN(appName, ".", 2)
	if len(splits) != 2 {
		span.SetStatus(codes.Error, "invalid name")
		return nil, fmt.Errorf("invalid package name '%s'", appName)
	}

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

// LoadModuleConf loads a module package from the given directory on the filesystem.
//
// Steps:
//  1. Validates the module directory exists
//  2. Loads the package definition (package.yaml, falling back to module.yaml)
//  3. Loads values (static values.yaml and OpenAPI schemas)
//  4. Discovers and loads batch hooks
//  5. Loads image digests (images_digests.json)
//
// Returns ErrPackageNotFound if the directory doesn't exist.
func LoadModuleConf(ctx context.Context, moduleDir string, logger *log.Logger) (*modules.Config, error) {
	ctx, span := otel.Tracer(loaderTracer).Start(ctx, "LoadModuleConf")
	defer span.End()

	span.SetAttributes(attribute.String("path", moduleDir))

	logger = logger.With(slog.String("path", moduleDir))

	logger.Debug("load module from directory")

	if _, err := os.Stat(moduleDir); os.IsNotExist(err) {
		span.SetStatus(codes.Error, ErrPackageNotFound.Error())
		return nil, ErrPackageNotFound
	}

	def, err := loadPackageDefinition(ctx, moduleDir)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load package from '%s': %w", moduleDir, err)
	}

	static, config, values, err := loadValues(def.Name, moduleDir)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load values: %w", err)
	}

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

// LoadGlobalConf loads the global module configuration from the globalPath directory.
// Unlike app and module loading, global hooks come from the compiled-in Go SDK registry,
// not from the filesystem. Only values and OpenAPI schemas are read from disk.
//
// Returns ErrPackageNotFound if the global-hooks directory doesn't exist.
func LoadGlobalConf(ctx context.Context, logger *log.Logger) (*global.Config, error) {
	ctx, span := otel.Tracer(loaderTracer).Start(ctx, "LoadGlobalConf")
	defer span.End()

	span.SetAttributes(attribute.String("path", globalPath))

	logger = logger.With(slog.String("path", globalPath))

	logger.Debug("load global module from directory")

	if _, err := os.Stat(globalPath); os.IsNotExist(err) {
		span.SetStatus(codes.Error, ErrPackageNotFound.Error())
		return nil, ErrPackageNotFound
	}

	static, config, values, err := loadValues("global", globalPath)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load values: %w", err)
	}

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

// loadPackageDefinition reads the package definition from the given directory.
// It first tries package.yaml (new format). If that file doesn't exist, it falls back
// to module.yaml (legacy format) and converts it to a dto.Definition, additionally
// resolving the version from the filesystem or dm-verity device.
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

// loadModuleDefinition reads and parses the legacy module.yaml file from the package directory.
// TODO(ipaqsa): remove when all modules are migrated to package.yaml
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

// loadDigests reads and parses images_digests.json from the package directory.
// Returns nil without error if the file doesn't exist (digests are optional).
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
