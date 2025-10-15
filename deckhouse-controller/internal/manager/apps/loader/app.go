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
	"os"
	"path/filepath"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/manager/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/manager/packages"
	"github.com/deckhouse/deckhouse/pkg/log"
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

		logger: logger.Named("application-loader"),
	}
}

func (l *ApplicationLoader) Load(ctx context.Context) (map[string]*apps.Application, error) {
	var instances []ApplicationInstance

	// TODO(ipaqsa): list instances in cluster

	result := make(map[string]*apps.Application)
	for _, inst := range instances {
		app, err := l.loadInstance(ctx, inst)
		if err != nil {
			return nil, fmt.Errorf("load application instance '%s/%s': %w", inst.Namespace, inst.Name, err)
		}

		result[inst.Name] = app
	}

	return result, nil
}

func (l *ApplicationLoader) loadInstance(_ context.Context, instance ApplicationInstance) (*apps.Application, error) {
	pkgPath := filepath.Join(l.appsDir, instance.Package)

	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		return nil, ErrPackageNotFound
	}

	pkgVersionPath := filepath.Join(pkgPath, instance.Version)
	if _, err := os.Stat(pkgVersionPath); os.IsNotExist(err) {
		return nil, ErrVersionNotFound
	}

	def, err := packages.LoadDefinition(pkgVersionPath)
	if err != nil {
		return nil, fmt.Errorf("load package '%s': %v", pkgVersionPath, err)
	}

	return def.ToApplication(instance.Name, instance.Namespace), nil
}
