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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/manager/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/manager/packages"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	appLoaderTracer = "application-loader"
)

var (
	ErrPackageNotFound = errors.New("package not found")
	ErrVersionNotFound = errors.New("package version not found")
)

type ApplicationLoader struct {
	cli     client.Client
	appsDir string

	logger *log.Logger
}

type ApplicationInstance struct {
	Name      string
	Namespace string
	Package   string
	Version   string
}

func NewApplicationLoader(cli client.Client, appsDir string, logger *log.Logger) *ApplicationLoader {
	return &ApplicationLoader{
		cli:     cli,
		appsDir: appsDir,

		logger: logger.Named(appLoaderTracer),
	}
}

// Load lists applications instances in the cluster and then matches them with packages on fs
func (l *ApplicationLoader) Load(ctx context.Context) (map[string]*apps.Application, error) {
	ctx, span := otel.Tracer(appLoaderTracer).Start(ctx, "Load")
	defer span.End()

	var instances []ApplicationInstance

	// TODO(ipaqsa): list instances in cluster

	span.SetAttributes(attribute.Int("found", len(instances)))

	span.SetAttributes(attribute.String("path", l.appsDir))
	l.logger.Debug("load applications from directory", slog.String("path", l.appsDir))

	result := make(map[string]*apps.Application)
	for _, inst := range instances {
		app, err := l.loadInstance(ctx, inst)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("load application instance '%s/%s': %w", inst.Namespace, inst.Name, err)
		}

		result[inst.Name] = app
	}

	return result, nil
}

// loadInstance matches application instance with package`s version on fs
func (l *ApplicationLoader) loadInstance(ctx context.Context, inst ApplicationInstance) (*apps.Application, error) {
	_, span := otel.Tracer(appLoaderTracer).Start(ctx, "loadInstance")
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

	// <apps>/<package>
	pkgPath := filepath.Join(l.appsDir, inst.Package)
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		span.SetStatus(codes.Error, ErrPackageNotFound.Error())
		return nil, ErrPackageNotFound
	}

	// <apps>/<package>/<version>
	pkgVersionPath := filepath.Join(pkgPath, inst.Version)
	if _, err := os.Stat(pkgVersionPath); os.IsNotExist(err) {
		span.SetStatus(codes.Error, ErrVersionNotFound.Error())
		return nil, ErrVersionNotFound
	}

	span.SetAttributes(attribute.String("path", pkgVersionPath))

	def, err := packages.LoadDefinition(pkgVersionPath)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("load package '%s': %v", pkgVersionPath, err)
	}

	return apps.NewApplication(pkgVersionPath, inst.Name, inst.Namespace, def), nil
}
