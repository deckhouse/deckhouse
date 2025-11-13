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

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	packagestatusservice "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status-package-service"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type PackageManager interface {
	PackageAdder
	PackageRemover
	PackageStatusChecker
}

type PackageStatusChecker interface {
	GetPackageStatus(ctx context.Context, packageName, namespace, version, packageType string) (PackageStatus, error)
}

type PackageStatus struct {
	Conditions []v1alpha1.ApplicationStatusCondition
}

type PackageAdder interface {
	AddApplication(ctx context.Context, app *v1alpha1.Application, apvStatus *v1alpha1.ApplicationPackageVersionStatus)
	AddClusterApplication(ctx context.Context, capvStatus *v1alpha1.ClusterApplicationPackageVersionStatus)
	AddModule(ctx context.Context, metadata *v1alpha1.ModuleReleaseSpec)
}

type PackageRemover interface {
	RemoveApplication(ctx context.Context, app *v1alpha1.Application)
	RemoveClusterApplication(ctx context.Context, capvStatus *v1alpha1.ClusterApplicationPackageVersionStatus)
	RemoveModule(ctx context.Context, metadata *v1alpha1.ModuleReleaseSpec)
}

type PackageOperator struct {
	logger       *log.Logger
	eventChannel chan<- packagestatusservice.PackageEvent
}

func NewPackageOperator(logger *log.Logger) *PackageOperator {
	return &PackageOperator{
		logger: logger,
	}
}

func (o *PackageOperator) SetEventChannel(ch chan<- packagestatusservice.PackageEvent) {
	o.eventChannel = ch
}

func (o *PackageOperator) AddApplication(_ context.Context, app *v1alpha1.Application, apvStatus *v1alpha1.ApplicationPackageVersionStatus) {
	o.logger.Debug("adding application",
		slog.String("name", app.Name),
		slog.String("namespace", app.Namespace),
		slog.String("package", apvStatus.PackageName),
		slog.String("version", apvStatus.Version))

	// initial event
	o.SendEvent(packagestatusservice.PackageEvent{
		PackageName: apvStatus.PackageName,
		Name:        app.Name,
		Namespace:   app.Namespace,
		Version:     apvStatus.Version,
		Type:        "application",
	})
}

func (o *PackageOperator) AddClusterApplication(_ context.Context, capvStatus *v1alpha1.ClusterApplicationPackageVersionStatus) {
	o.logger.Debug("adding cluster application", slog.String("name", capvStatus.PackageName), slog.String("version", capvStatus.Version))
}

func (o *PackageOperator) AddModule(_ context.Context, metadata *v1alpha1.ModuleReleaseSpec) {
	o.logger.Debug("adding module", slog.String("name", metadata.ModuleName), slog.String("version", metadata.Version))
}

func (o *PackageOperator) RemoveApplication(_ context.Context, app *v1alpha1.Application) {
	o.logger.Debug("removing application", slog.String("name", app.Name))
}

func (o *PackageOperator) RemoveClusterApplication(_ context.Context, capvStatus *v1alpha1.ClusterApplicationPackageVersionStatus) {
	o.logger.Debug("removing cluster application", slog.String("name", capvStatus.PackageName), slog.String("version", capvStatus.Version))
}

func (o *PackageOperator) RemoveModule(_ context.Context, metadata *v1alpha1.ModuleReleaseSpec) {
	o.logger.Debug("removing module", slog.String("name", metadata.ModuleName), slog.String("version", metadata.Version))
}

func (o *PackageOperator) GetPackageStatus(_ context.Context, packageName, namespace, version, packageType string) (PackageStatus, error) {
	o.logger.Debug("getting package status",
		slog.String("package", packageName),
		slog.String("namespace", namespace),
		slog.String("version", version),
		slog.String("type", packageType))

	return PackageStatus{
		Conditions: []v1alpha1.ApplicationStatusCondition{},
	}, nil
}

func (o *PackageOperator) SendEvent(event packagestatusservice.PackageEvent) {
	if o.eventChannel != nil {
		select {
		case o.eventChannel <- event:
		default:
			o.logger.Warn("event channel is full, dropping event", slog.Any("event", event))
		}
	}
}
