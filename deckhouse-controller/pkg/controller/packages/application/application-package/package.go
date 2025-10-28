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

package applicationpackage

import (
	"context"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PackageAdder interface {
	AddApplication(ctx context.Context, metadata *v1alpha1.ApplicationPackageVersionStatusMetadata)
	AddClusterApplication(ctx context.Context, metadata *v1alpha1.ClusterApplicationPackageVersionStatusMetadata)
	AddModule(ctx context.Context, metadata *v1alpha1.ModuleReleaseSpec)
}

type PackageOperator struct {
	client client.Client
	logger *log.Logger
}

func NewPackageOperator(client client.Client, logger *log.Logger) *PackageOperator {
	return &PackageOperator{
		client: client,
		logger: logger,
	}
}

func (o *PackageOperator) AddApplication(ctx context.Context, metadata *v1alpha1.ApplicationPackageVersionStatusMetadata) {
}

func (o *PackageOperator) AddClusterApplication(ctx context.Context, metadata *v1alpha1.ClusterApplicationPackageVersionStatusMetadata) {
}

func (o *PackageOperator) AddModule(ctx context.Context, metadata *v1alpha1.ModuleReleaseSpec) {
}
