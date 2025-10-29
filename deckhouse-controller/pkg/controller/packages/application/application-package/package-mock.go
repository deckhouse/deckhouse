/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package applicationpackage

import (
	"context"
	"log/slog"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type PackageOperatorStub struct {
	client client.Client
	logger *log.Logger
}

func NewStubPackageOperator(client client.Client, logger *log.Logger) *PackageOperatorStub {
	return &PackageOperatorStub{
		client: client,
		logger: logger,
	}
}

func (m *PackageOperatorStub) AddApplication(_ context.Context, apvStatus *v1alpha1.ApplicationPackageVersionStatus) {
	m.logger.Debug("adding application", slog.String("name", apvStatus.PackageName), slog.String("version", apvStatus.Version))
}

func (m *PackageOperatorStub) AddClusterApplication(_ context.Context, capvStatus *v1alpha1.ClusterApplicationPackageVersionStatus) {
	m.logger.Debug("adding cluster application", slog.String("name", capvStatus.PackageName), slog.String("version", capvStatus.Version))
}

func (m *PackageOperatorStub) AddModule(_ context.Context, metadata *v1alpha1.ModuleReleaseSpec) {
	m.logger.Debug("adding module", slog.String("name", metadata.ModuleName), slog.String("version", metadata.Version))
}

func (m *PackageOperatorStub) RemoveApplication(_ context.Context, app *v1alpha1.Application) {
	m.logger.Debug("removing application", slog.String("name", app.Name))
}

func (m *PackageOperatorStub) RemoveClusterApplication(_ context.Context, capvStatus *v1alpha1.ClusterApplicationPackageVersionStatus) {
	m.logger.Debug("removing cluster application", slog.String("name", capvStatus.PackageName), slog.String("version", capvStatus.Version))
}

func (m *PackageOperatorStub) RemoveModule(_ context.Context, metadata *v1alpha1.ModuleReleaseSpec) {
	m.logger.Debug("removing module", slog.String("name", metadata.ModuleName), slog.String("version", metadata.Version))
}
