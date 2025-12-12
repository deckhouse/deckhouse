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

	shapp "github.com/flant/shell-operator/pkg/app"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/dto"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	appLoaderTracer = "application-loader"

	digestsFile = "images_digests.json"
)

var (
	// ErrPackageNotFound is returned when the requested package directory doesn't exist
	ErrPackageNotFound = errors.New("package not found")
)

// ApplicationLoader loads application packages from the filesystem.
// It validates package structure, loads definitions, values, and hooks.
type ApplicationLoader struct {
	appsDir string

	logger *log.Logger
}

// ApplicationInstance represents a deployed application instance.
// It contains the metadata needed to locate and load the corresponding package.
type ApplicationInstance struct {
	Name      string // Unique name of the application instance
	Namespace string // Kubernetes namespace where the application is deployed
	Package   string // Package name (directory name under appsDir)
	Version   string // Package version (directory name under package)
}

// NewApplicationLoader creates a new ApplicationLoader for the specified directory.
// The appsDir should contain package directories organized as: <package>/<version>/
func NewApplicationLoader(appsDir string, logger *log.Logger) *ApplicationLoader {
	return &ApplicationLoader{
		appsDir: appsDir,

		logger: logger.Named(appLoaderTracer),
	}
}

// Load loads an application package from the filesystem based on the instance specification.
// It performs the following steps:
//  1. Validates package and version directories exist
//  2. Loads package definition (package.yaml) - currently not used
//  3. Loads values (static values.yaml and OpenAPI schemas)
//  4. Discovers and loads hooks (shell and batch)
//  5. Creates and returns an Application instance
//
// Returns ErrPackageNotFound if package directory doesn't exist.
// Returns ErrVersionNotFound if version directory doesn't exist.
func (l *ApplicationLoader) Load(ctx context.Context, reg registry.Registry, name string) (*apps.Application, error) {
	_, span := otel.Tracer(appLoaderTracer).Start(ctx, "Load")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))

	logger := l.logger.With(slog.String("name", name))

	logger.Debug("load application from directory", slog.String("path", l.appsDir))

	// Verify package directory exists: <apps>/<package>
	path := filepath.Join(l.appsDir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		span.SetStatus(codes.Error, ErrPackageNotFound.Error())
		return nil, ErrPackageNotFound
	}

	span.SetAttributes(attribute.String("path", path))

	// Load package definition (package.yaml)
	def, err := loadDefinition(path)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load package from '%s': %w", path, err)
	}

	// Load values from values.yaml and openapi schemas
	static, config, values, err := loadValues(def.Name, path)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load values: %w", err)
	}

	// Discover and load hooks (shell and batch)
	hooksLoader := newHookLoader(name, path, shapp.DebugKeepTmpFiles, l.logger)
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

	digests, err := loadDigests(path)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load digests: %w", err)
	}

	// Build application configuration
	conf := apps.ApplicationConfig{
		Definition: appDef,

		Digests:  digests,
		Registry: reg,

		StaticValues: static,
		ConfigSchema: config,
		ValuesSchema: values,

		Hooks: hooks,

		SettingsCheck: hooksLoader.settingsCheck,
	}

	app, err := apps.NewApplication(name, path, conf)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("create new application: %w", err)
	}

	return app, nil
}

// loadDefinition reads and parses the package.yaml file from the package directory.
// It validates YAML structure but doesn't validate content.
//
// Returns the parsed Definition or an error if reading or parsing fails.
func loadDefinition(packageDir string) (*dto.Definition, error) {
	path := filepath.Join(packageDir, dto.DefinitionFile)

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file '%s': %w", path, err)
	}

	def := new(dto.Definition)
	if err = yaml.Unmarshal(content, def); err != nil {
		return nil, fmt.Errorf("unmarshal file '%s': %w", path, err)
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
