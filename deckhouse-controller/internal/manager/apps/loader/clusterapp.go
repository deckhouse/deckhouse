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
	"fmt"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/manager/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/manager/packages"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	clusterAppLoaderTracer = "cluster-app-loader"
)

type ClusterAppLoader struct {
	appsDir string

	logger *log.Logger
}

func NewClusterAppLoader(appsDir string, logger *log.Logger) *ClusterAppLoader {
	return &ClusterAppLoader{
		appsDir: appsDir,

		logger: logger.Named(clusterAppLoaderTracer),
	}
}

// Load traverses over apps dir and loads cluster applications from packages
func (l *ClusterAppLoader) Load(ctx context.Context) (map[string]*apps.ClusterApplication, error) {
	ctx, span := otel.Tracer(clusterAppLoaderTracer).Start(ctx, "Load")
	defer span.End()

	span.SetAttributes(attribute.String("path", l.appsDir))

	entries, err := os.ReadDir(l.appsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		span.SetStatus(codes.Error, err.Error())

		return nil, fmt.Errorf("read directory '%s': %v", l.appsDir, err)
	}

	span.SetAttributes(attribute.Int("found", len(entries)))

	res := make(map[string]*apps.ClusterApplication, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		appPath := filepath.Join(l.appsDir, entry.Name())
		def, err := packages.LoadDefinition(appPath)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("load package '%s': %v", appPath, err)
		}

		res[def.Name] = apps.NewClusterApplication(appPath, def)
	}

	return res, nil
}
