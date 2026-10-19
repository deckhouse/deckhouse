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
	"encoding/json"
	"fmt"
	"log/slog"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1beta1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Service processes status events and updates Module conditions.
type Service struct {
	client client.Client
	source source
	logger *log.Logger
}

// source is the package engine's status: the lifecycle Report and the observations.
type source interface {
	Report(name string) (model.Report, bool)
	Observed(name string) observed.Observation
}

// NewService creates the status service of the kind.
func NewService(client client.Client, source source, logger *log.Logger) *Service {
	return &Service{
		client: client,
		source: source,
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
				// A conflict is a patch from a stale cached copy (the optimistic lock): expected while
				// notifications for one name arrive in bursts.
				if apierrors.IsConflict(err) {
					s.logger.Debug("status patch conflict, requeued", slog.String("name", name), log.Err(err))
				} else {
					s.logger.Warn("handle status event, requeued", slog.String("name", name), log.Err(err))
				}

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
	module := new(v1beta1.Module)
	if err := s.client.Get(ctx, client.ObjectKey{Name: name}, module); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get module: %w", err)
	}

	original := module.DeepCopy()

	// Get the package status from the operator and compute conditions
	s.computeAndApplyConditions(name, module)

	// The resync republishes every package on a timer, so most events compute a
	// status identical to the published one. Patching it anyway would bump
	// resourceVersion on every tick and wake every watcher for nothing.
	if apiequality.Semantic.DeepEqual(original.Status, module.Status) {
		return nil
	}

	// The projection reads the conditions already on the resource (Installed is sticky), so a patch
	// computed from a stale cached copy must fail rather than replace the conditions: the optimistic
	// lock turns it into a conflict, and the name is requeued.
	patch := client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{})
	if err := s.client.Status().Patch(ctx, module, patch); err != nil {
		return fmt.Errorf("patch module status: %w", err)
	}

	return nil
}

func (s *Service) computeAndApplyConditions(name string, module *v1beta1.Module) {
	report, known := s.source.Report(name)
	obs := s.source.Observed(name)

	if module.Status.CurrentVersion == nil {
		module.Status.CurrentVersion = new(v1beta1.ModuleStatusVersion)
	}

	// The CR is the authority on deletion: the engine forgets a removed package, and never
	// knows one after a restart mid-deletion.
	deleting := !module.DeletionTimestamp.IsZero()

	f := buildFacts(input{
		Name:           name,
		Report:         report,
		Known:          known,
		Observation:    obs,
		External:       module.Status.Conditions,
		CurrentVersion: module.Status.CurrentVersion.Version,
		Deleting:       deleting,
	})

	m := mapConditions(&f)
	for _, cond := range m.set {
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

	// Gated conditions are not reported while their gate is closed: until the
	// module is installed, Installed is the only condition it carries.
	for _, condType := range m.remove {
		meta.RemoveStatusCondition(&module.Status.Conditions, condType)
	}

	// Nothing about a live install is committed onto a resource on its way out.
	if !deleting && known {
		// The activated version: Boot restores it, so it is written at activation.
		if report.CurrentVersion != "" {
			module.Status.CurrentVersion.Version = report.CurrentVersion
		}

		// The settings checkpoint Boot restores, as effective values.
		if values := report.Current.AppliedValues; values != nil {
			if raw, err := json.Marshal(values); err == nil {
				module.Status.LastAppliedConfiguration = runtime.RawExtension{Raw: raw}
			}
		}
	}

	if enabled, ok := schedulerVerdict(&f); ok {
		module.Status.Enabled = &enabled
	}

	// An empty progress leaves the previous tracking on the CR.
	if !obs.Tracking.Empty() {
		if raw, err := json.Marshal(obs.Tracking); err == nil {
			module.Status.Tracking = runtime.RawExtension{Raw: raw}
		}
	}

	// Summary is computed from the same facts the mappers read.
	summary := summarize(&f)
	module.Status.Summary = &v1beta1.ModuleStatusSummary{
		State:   summary.state,
		Message: summary.message,
		Tip:     summary.tip,
	}
}
