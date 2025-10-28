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
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	appLoaderTracer = "application-loader"
)

var (
	// ErrPackageNotFound is returned when the requested package directory doesn't exist
	ErrPackageNotFound = errors.New("package not found")
	// ErrVersionNotFound is returned when the requested package version directory doesn't exist
	ErrVersionNotFound = errors.New("package version not found")
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
func (l *ApplicationLoader) Load(ctx context.Context, inst ApplicationInstance) (*apps.Application, error) {
	_, span := otel.Tracer(appLoaderTracer).Start(ctx, "Load")
	defer span.End()

	span.SetAttributes(attribute.String("name", inst.Name))
	span.SetAttributes(attribute.String("namespace", inst.Namespace))
	span.SetAttributes(attribute.String("package", inst.Package))
	span.SetAttributes(attribute.String("version", inst.Version))

	logger := l.logger.With(
		slog.String("name", inst.Name),
		slog.String("namespace", inst.Namespace),
		slog.String("package", inst.Package),
		slog.String("version", inst.Version))

	logger.Debug("load application from directory", slog.String("path", l.appsDir))

	// Verify package directory exists: <apps>/<package>
	pkgPath := filepath.Join(l.appsDir, inst.Package)
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		span.SetStatus(codes.Error, ErrPackageNotFound.Error())
		return nil, ErrPackageNotFound
	}

	// Verify package version directory exists: <apps>/<package>/<version>
	pkgVersionPath := filepath.Join(pkgPath, inst.Version)
	if _, err := os.Stat(pkgVersionPath); os.IsNotExist(err) {
		span.SetStatus(codes.Error, ErrVersionNotFound.Error())
		return nil, ErrVersionNotFound
	}

	span.SetAttributes(attribute.String("path", pkgVersionPath))

	// Load package definition (package.yaml)
	// TODO: Validate that definition matches requested package/version
	_, err := loadDefinition(pkgVersionPath)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load package '%s': %w", pkgVersionPath, err)
	}

	// Load values from values.yaml and openapi schemas
	static, config, values, err := loadValues(inst.Name, pkgVersionPath)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load values: %w", err)
	}

	// Discover and load hooks (shell and batch)
	hooksLoader := newHookLoader(inst.Name, pkgVersionPath, shapp.DebugKeepTmpFiles, l.logger)
	hooks, err := hooksLoader.load(ctx)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load hooks: %w", err)
	}

	// Build application configuration
	conf := apps.ApplicationConfig{
		Namespace: inst.Namespace,

		PackageName: inst.Package,

		StaticValues: static,
		ConfigSchema: config,
		ValuesSchema: values,

		Hooks: hooks,
	}

	app, err := apps.NewApplication(inst.Name, conf)
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
