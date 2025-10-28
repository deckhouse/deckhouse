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
