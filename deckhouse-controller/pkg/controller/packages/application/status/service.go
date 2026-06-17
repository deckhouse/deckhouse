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

package status

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/condmap"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Service processes status events and updates Application conditions.
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
// package names from the queue and reflects them onto Application resources.
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

// handleEvent reflects a package status change onto its Application resource.
// Event format is "namespace.name". A returned error is retryable; nil means
// done — including a malformed name or a missing Application, which never
// become valid on retry.
func (s *Service) handleEvent(ctx context.Context, ev string) error {
	logger := s.logger.With(slog.String("name", ev))

	// Parse event name: "namespace.name"
	splits := strings.Split(ev, ".")
	if len(splits) != 2 {
		logger.Warn("invalid event format, expected 'namespace.name'")
		return nil
	}

	// Fetch the Application resource
	app := new(v1alpha1.Application)
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: splits[0], Name: splits[1]}, app); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get application: %w", err)
	}

	original := app.DeepCopy()

	// Get the package status from the operator and compute conditions
	s.computeAndApplyConditions(ev, app)

	if err := s.client.Status().Patch(ctx, app, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch application status: %w", err)
	}

	return nil
}

func (s *Service) computeAndApplyConditions(ev string, app *v1alpha1.Application) {
	packageStatus := s.getter(ev)

	if app.Status.CurrentVersion == nil {
		app.Status.CurrentVersion = new(v1alpha1.ApplicationStatusVersion)
	}

	versionChanged := app.Status.CurrentVersion.Version != "" && app.Status.CurrentVersion.Version != packageStatus.Version
	mapperStatus := s.buildMapperStatus(versionChanged, app.Status.Conditions, packageStatus.Conditions)

	// Apply mapped conditions (external user-facing conditions)
	for _, cond := range s.mapper.Map(mapperStatus) {
		// Reason is required by metav1.Condition contract
		reason := cond.Reason
		if reason == "" {
			reason = cond.Type
		}

		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type:               cond.Type,
			Status:             cond.Status,
			Reason:             reason,
			Message:            cond.Message,
			ObservedGeneration: app.Generation,
		})
	}

	if packageStatus.IsConditionTrue(status.ConditionManifestsApplied) {
		app.Status.CurrentVersion.Version = packageStatus.Version

		if packageStatus.Settings != nil {
			if raw, err := json.Marshal(packageStatus.Settings); err == nil {
				app.Status.LastAppliedConfiguration = runtime.RawExtension{Raw: raw}
			}
		}
	}

	// Skip writing tracking if there's nothing to report — preserves the previous
	// tracking field on the CR through trailing empty progress events from nelm.
	if len(packageStatus.Tracking.Report.Operations) > 0 {
		raw, _ := json.Marshal(packageStatus.Tracking)
		app.Status.Tracking = runtime.RawExtension{Raw: raw}
	}

	// Summary is computed from the same pre-mapping state the mapper consumed,
	// not from the merged conditions: summarize shares the mapper's phase and
	// dependency-disabled helpers, so the two cannot drift, and reads the
	// internal conditions directly instead of reverse-deriving reasons.
	state, message, tip := summarize(mapperStatus)
	app.Status.Summary = &v1alpha1.ApplicationStatusSummary{
		State:   state,
		Message: message,
		Tip:     tip,
	}
}

// buildMapperStatus creates mapper input from Application and internal conditions.
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
