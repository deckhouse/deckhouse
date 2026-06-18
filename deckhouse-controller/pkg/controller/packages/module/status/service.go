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

// Package status reflects the new package runtime's internal status onto the
// legacy v1alpha1.Module custom resource. It mirrors the application status
// service but targets the cluster-scoped Module CR that is currently in use,
// so existing consumers of the module status keep working during the migration
// off addon-operator.
package status

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type getter func(name string) status.Status

// Service consumes runtime status-change notifications and reflects them onto
// v1alpha1.Module resources.
type Service struct {
	client client.Client
	getter getter
	logger *log.Logger
}

// NewService creates a module status service. getter resolves the current
// internal runtime status for a module by name (the same name the runtime keys
// modules and status notifications by).
func NewService(client client.Client, getter getter, logger *log.Logger) *Service {
	return &Service{
		client: client,
		getter: getter,
		logger: logger.Named("module-status-service"),
	}
}

// Start begins the status service event loop in a goroutine. It pulls changed
// module names from the queue and reflects them onto Module resources. The loop
// exits when the queue is shut down.
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

// handleEvent reflects a module status change onto its Module resource. The
// runtime keys modules by their bare name, so dotted names (application events
// of the form "namespace.name") are ignored: they are never valid Module names
// and never become valid on retry.
func (s *Service) handleEvent(ctx context.Context, name string) error {
	if strings.Contains(name, ".") {
		return nil
	}

	logger := s.logger.With(slog.String("name", name))

	module := new(v1alpha1.Module)
	if err := s.client.Get(ctx, client.ObjectKey{Name: name}, module); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("module not found, skip status update")
			return nil
		}
		return fmt.Errorf("get module: %w", err)
	}

	original := module.DeepCopy()

	s.applyStatus(name, module)

	if err := s.client.Status().Patch(ctx, module, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch module status: %w", err)
	}

	return nil
}

// applyStatus maps the internal runtime conditions onto the Module's phase and
// conditions, aiming for parity with the addon-operator-driven status writer
// (module-controllers/config/status.go).
func (s *Service) applyStatus(name string, module *v1alpha1.Module) {
	st := s.getter(name)

	phase, ready, reason, message := computeModuleStatus(st)

	module.Status.Phase = phase

	// A module owned by the new runtime is, by definition, enabled by the
	// scheduler (only enabled modules are scheduled/adopted).
	module.SetConditionTrue(v1alpha1.ModuleConditionEnabledByModuleManager)

	if ready {
		module.SetConditionTrue(v1alpha1.ModuleConditionIsReady)
		return
	}

	module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, reason, message)
}

// pipelineConditions are the internal conditions that gate a module becoming
// ready, in execution order. The first one that is explicitly False marks the
// module as not-ready (and, when it carries a reason, as errored).
var pipelineConditions = []status.ConditionType{
	status.ConditionRequirementsMet,
	status.ConditionReadyOnFilesystem,
	status.ConditionLoaded,
	status.ConditionCRDsEnsured,
	status.ConditionConfigured,
	status.ConditionHooksProcessed,
	status.ConditionManifestsApplied,
}

// computeModuleStatus reduces the internal runtime status to a Module phase,
// readiness flag and a reason/message for the IsReady condition.
//
//   - Any gating condition that is explicitly False means the module is not
//     ready. If that condition carries a reason it is treated as an error
//     (phase Error); otherwise the module is still reconciling.
//   - Ready requires manifests applied and hooks processed, with workloads
//     either scaled or not yet observed (Unknown).
func computeModuleStatus(st status.Status) (phase string, ready bool, reason, message string) {
	byType := make(map[status.ConditionType]status.Condition, len(st.Conditions))
	for _, c := range st.Conditions {
		byType[c.Type] = c
	}

	for _, condType := range pipelineConditions {
		c, ok := byType[condType]
		if !ok || c.Status != metav1.ConditionFalse {
			continue
		}

		if c.Reason != "" {
			return v1alpha1.ModulePhaseError, false, string(c.Reason), c.Message
		}

		return v1alpha1.ModulePhaseReconciling, false, v1alpha1.ModuleReasonReconciling, v1alpha1.ModuleMessageReconciling
	}

	manifestsApplied := byType[status.ConditionManifestsApplied].Status == metav1.ConditionTrue
	hooksProcessed := byType[status.ConditionHooksProcessed].Status == metav1.ConditionTrue
	scaledOK := byType[status.ConditionScaled].Status != metav1.ConditionFalse

	if manifestsApplied && hooksProcessed && scaledOK {
		return v1alpha1.ModulePhaseReady, true, "", ""
	}

	return v1alpha1.ModulePhaseReconciling, false, v1alpha1.ModuleReasonReconciling, v1alpha1.ModuleMessageReconciling
}
