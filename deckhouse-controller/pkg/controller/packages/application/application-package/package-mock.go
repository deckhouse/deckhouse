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

type PackageOperatorMock struct {
	client client.Client
	logger *log.Logger
}

func NewMockedPackageOperator(client client.Client, logger *log.Logger) *PackageOperatorMock {
	return &PackageOperatorMock{
		client: client,
		logger: logger,
	}
}

func (m *PackageOperatorMock) AddApplication(_ context.Context, apvStatus *v1alpha1.ApplicationPackageVersionStatus) {
	m.logger.Debug("adding application", slog.String("name", apvStatus.PackageName), slog.String("version", apvStatus.Version))
}

func (m *PackageOperatorMock) AddClusterApplication(_ context.Context, capvStatus *v1alpha1.ClusterApplicationPackageVersionStatus) {
	m.logger.Debug("adding cluster application", slog.String("name", capvStatus.PackageName), slog.String("version", capvStatus.Version))
}

func (m *PackageOperatorMock) AddModule(_ context.Context, metadata *v1alpha1.ModuleReleaseSpec) {
	m.logger.Debug("adding module", slog.String("name", metadata.ModuleName), slog.String("version", metadata.Version))
}
