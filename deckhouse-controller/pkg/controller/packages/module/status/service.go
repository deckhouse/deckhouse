// Copyright 2026 Flant JSC
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

package status

import (
	"context"
	"fmt"
	"log/slog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/condmap"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Service processes status events and updates Module conditions.
type Service struct {
	client client.Client
	getter getter
	mapper condmap.Mapper
	logger *log.Logger
}

type getter func(name string) status.Status

// NewService creates a new status service with default condition specs.
func NewService(client client.Client, getter getter, logger *log.Logger) *Service {
	return &Service{
		client: client,
		getter: getter,
		mapper: buildMapper(),
		logger: logger.Named("status-service"),
	}
}

// Start begins the status service event loop in a goroutine. It pulls changed
// package names from the queue and reflects them onto Module resources.
// The loop exits when the queue is shut down.
func (s *Service) Start(ctx context.Context, queue workqueue.TypedRateLimitingInterface[string]) {
	go func() {
		for {
			name, shutdown := queue.Get()
			if shutdown {
				return
			}

			if err := s.handleEvent(ctx, name); err != nil {
				s.logger.Warn("handle status event, requeued", slog.String("name", name), log.Err(err))
				queue.AddRateLimited(name)
			} else {
				queue.Forget(name)
			}

			queue.Done(name)
		}
	}()
}

// handleEvent reflects a package status change onto its Module resource.
// The event is a plain module name. A returned error is retryable; nil means
// done — including a missing Module, which never becomes valid on retry.
func (s *Service) handleEvent(ctx context.Context, name string) error {
	module := new(v1alpha2.Module)
	if err := s.client.Get(ctx, client.ObjectKey{Name: name}, module); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get module: %w", err)
	}

	original := module.DeepCopy()

	// Get the package status from the operator and compute conditions
	s.computeAndApplyConditions(name, module)

	if err := s.client.Status().Patch(ctx, module, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch module status: %w", err)
	}

	return nil
}

func (s *Service) computeAndApplyConditions(name string, module *v1alpha2.Module) {
	packageStatus := s.getter(name)

	if module.Status.CurrentVersion == nil {
		module.Status.CurrentVersion = new(v1alpha2.ModuleStatusVersion)
	}

	versionChanged := module.Status.CurrentVersion.Version != "" && module.Status.CurrentVersion.Version != packageStatus.Version
	mapperStatus := s.buildMapperStatus(versionChanged, module.Status.Conditions, packageStatus.Conditions)

	// Apply mapped conditions (external user-facing conditions)
	for _, cond := range s.mapper.Map(mapperStatus) {
		// Reason is required by metav1.Condition contract
		reason := cond.Reason
		if reason == "" {
			reason = cond.Type
		}

		meta.SetStatusCondition(&module.Status.Conditions, metav1.Condition{
			Type:               cond.Type,
			Status:             cond.Status,
			Reason:             reason,
			Message:            cond.Message,
			ObservedGeneration: module.Generation,
		})
	}

	if packageStatus.IsConditionTrue(status.ConditionManifestsApplied) {
		module.Status.CurrentVersion.Version = packageStatus.Version
	}
}

// buildMapperStatus creates mapper input from Module and internal conditions.
func (s *Service) buildMapperStatus(versionChanged bool, external []metav1.Condition, internal []status.Condition) condmap.State {
	mapperStatus := condmap.State{
		External: make(map[string]metav1.Condition, len(external)),
		Internal: make(map[string]metav1.Condition, len(internal)),
	}

	for _, cond := range internal {
		mapperStatus.Internal[string(cond.Type)] = metav1.Condition{
			Type:    string(cond.Type),
			Status:  cond.Status,
			Reason:  string(cond.Reason),
			Message: cond.Message,
		}
	}

	for _, cond := range external {
		mapperStatus.External[cond.Type] = metav1.Condition{
			Type:    cond.Type,
			Status:  cond.Status,
			Reason:  cond.Reason,
			Message: cond.Message,
		}
	}

	mapperStatus.Updating = versionChanged

	return mapperStatus
}
