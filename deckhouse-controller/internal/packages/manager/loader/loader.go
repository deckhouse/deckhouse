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

	shapp "github.com/flant/shell-operator/pkg/app"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/dto"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/apps"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	loaderTracer = "package-loader"

	digestsFile = "images_digests.json"
)

var (
	// ErrPackageNotFound is returned when the requested package directory doesn't exist
	ErrPackageNotFound = errors.New("package not found")
)

// LoadAppConf loads an application package from the filesystem based on the instance specification.
// It performs the following steps:
//  1. Validates package and version directories exist
//  2. Loads package definition (package.yaml) - currently not used
//  3. Loads values (static values.yaml and OpenAPI schemas)
//  4. Discovers and loads hooks (shell and batch)
//  5. Creates and returns an Application instance
//
// Returns ErrPackageNotFound if package directory doesn't exist.
func LoadAppConf(ctx context.Context, packageDir string, logger *log.Logger) (*apps.Config, error) {
	ctx, span := otel.Tracer(loaderTracer).Start(ctx, "LoadAppConf")
	defer span.End()

	span.SetAttributes(attribute.String("path", packageDir))

	logger = logger.With(slog.String("path", packageDir))

	logger.Debug("load application from directory", slog.String("path", packageDir))

	if _, err := os.Stat(packageDir); os.IsNotExist(err) {
		span.SetStatus(codes.Error, ErrPackageNotFound.Error())
		return nil, ErrPackageNotFound
	}

	span.SetAttributes(attribute.String("path", packageDir))

	// Load package definition (package.yaml)
	def, err := loadDefinition(packageDir)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load package from '%s': %w", packageDir, err)
	}

	// Load values from values.yaml and openapi schemas
	static, config, values, err := loadValues(def.Name, packageDir)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load values: %w", err)
	}

	packageName := filepath.Base(packageDir)

	splits := strings.SplitN(packageName, ".", 2)
	if len(splits) != 2 {
		span.SetStatus(codes.Error, "invalid name")
		return nil, fmt.Errorf("invalid package name '%s'", packageName)
	}

	// Discover and load hooks (shell and batch)
	hooksLoader := newHookLoader(splits[0], splits[1], packageDir, shapp.DebugKeepTmpFiles, logger)
	hooks, err := hooksLoader.load(ctx)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load hooks: %w", err)
	}

	appDef, err := def.ToApplication()
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("convert app definition: %w", err)
	}

	digests, err := loadDigests(packageDir)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load digests: %w", err)
	}

	return &apps.Config{
		Path:       packageDir,
		Definition: appDef,

		Digests: digests,

		StaticValues: static,
		ConfigSchema: config,
		ValuesSchema: values,

		Hooks: hooks,

		SettingsCheck: hooksLoader.settingsCheck,
	}, nil
}

// loadDefinition reads and parses the package.yaml file from the package directory.
// It validates YAML structure but doesn't validate content.
//
// Returns the parsed Definition or an error if reading or parsing fails.
func loadDefinition(packageDir string) (*dto.Definition, error) {
	definitionPath := filepath.Join(packageDir, dto.DefinitionFile)

	content, err := os.ReadFile(definitionPath)
	if err != nil {
		return nil, fmt.Errorf("read definition file '%s': %w", definitionPath, err)
	}

	def := new(dto.Definition)
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
