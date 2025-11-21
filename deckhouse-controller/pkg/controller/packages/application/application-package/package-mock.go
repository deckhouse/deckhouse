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
	packagestatusservice "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status-package-service"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type StatusServiceInterface interface {
	HandleEvent(ctx context.Context, event packagestatusservice.PackageEvent)
}

type PackageOperatorStub struct {
	client        client.Client
	logger        *log.Logger
	eventChannel  chan<- packagestatusservice.PackageEvent
	statusService StatusServiceInterface
}

func NewStubPackageOperator(client client.Client, logger *log.Logger) *PackageOperatorStub {
	return &PackageOperatorStub{
		client: client,
		logger: logger,
	}
}

func (m *PackageOperatorStub) SetEventChannel(ch chan<- packagestatusservice.PackageEvent) {
	m.eventChannel = ch
}

func (m *PackageOperatorStub) SetStatusService(ss StatusServiceInterface) {
	m.statusService = ss
}

func (m *PackageOperatorStub) AddApplication(_ context.Context, app *v1alpha1.Application, apvStatus *v1alpha1.ApplicationPackageVersionStatus) {
	m.logger.Debug("adding application",
		slog.String("name", app.Name),
		slog.String("namespace", app.Namespace),
		slog.String("package", apvStatus.PackageName),
		slog.String("version", apvStatus.Version))

	event := packagestatusservice.PackageEvent{
		PackageName: apvStatus.PackageName,
		Name:        app.Name,
		Namespace:   app.Namespace,
		Version:     apvStatus.Version,
		Type:        "application",
	}

	if m.statusService != nil {
		m.statusService.HandleEvent(context.Background(), event)
	} else {
		m.SendEvent(event)
	}
}

func (m *PackageOperatorStub) AddModule(_ context.Context, metadata *v1alpha1.ModuleReleaseSpec) {
	m.logger.Debug("adding module", slog.String("name", metadata.ModuleName), slog.String("version", metadata.Version))
}

func (m *PackageOperatorStub) RemoveApplication(_ context.Context, app *v1alpha1.Application) {
	m.logger.Debug("removing application", slog.String("name", app.Name))
}

func (m *PackageOperatorStub) RemoveModule(_ context.Context, metadata *v1alpha1.ModuleReleaseSpec) {
	m.logger.Debug("removing module", slog.String("name", metadata.ModuleName), slog.String("version", metadata.Version))
}

func (m *PackageOperatorStub) GetPackageStatus(_ context.Context, packageName, namespace, version, packageType string) (PackageStatus, error) {
	m.logger.Debug("getting package status",
		slog.String("package", packageName),
		slog.String("namespace", namespace),
		slog.String("version", version),
		slog.String("type", packageType))

	if version == "v1.0.1" {
		return PackageStatus{
			Conditions: []v1alpha1.ApplicationStatusCondition{
				{
					Status: "True",
					Type:   "Processed",
				},
			},
			InternalConditions: []v1alpha1.ApplicationInternalStatusCondition{
				{
					Status: "True",
					Type:   "UpdateNotified",
				},
			},
		}, nil
	}

	if version == "v1.0.2" {
		return PackageStatus{
			Conditions: []v1alpha1.ApplicationStatusCondition{
				{
					Status: "True",
					Type:   "Processed",
				},
				{
					Status:  "False",
					Type:    "UpdateAvailable",
					Reason:  "NewerVersionAvailable",
					Message: "A newer version v1.0.2 is available.",
				},
			},
			InternalConditions: []v1alpha1.ApplicationInternalStatusCondition{
				{
					Status: "True",
					Type:   "UpdateNotified",
				},
			},
		}, nil
	}

	if version == "v1.0.3" {
		return PackageStatus{
			Conditions: []v1alpha1.ApplicationStatusCondition{
				{
					Status: "False",
					Type:   "Processed",
				},
				{
					Status:  "False",
					Type:    "SomeCriticalCondition",
					Reason:  "CriticalIssueDetected",
					Message: "A critical issue has been detected in version v1.0.2.",
				},
			},
			InternalConditions: []v1alpha1.ApplicationInternalStatusCondition{
				{
					Status: "False",
					Type:   "CriticalConditionMet",
				},
			},
		}, nil
	}

	return PackageStatus{
		Conditions:         []v1alpha1.ApplicationStatusCondition{},
		InternalConditions: []v1alpha1.ApplicationInternalStatusCondition{},
	}, nil
}

func (m *PackageOperatorStub) SendEvent(event packagestatusservice.PackageEvent) {
	if m.eventChannel != nil {
		m.eventChannel <- event
	}
}
