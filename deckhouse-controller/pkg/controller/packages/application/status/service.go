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

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Service processes status events and updates Application conditions.
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

	// The resync republishes every package on a timer, so most events compute a
	// status identical to the published one. Patching it anyway would bump
	// resourceVersion on every tick and wake every watcher for nothing.
	if apiequality.Semantic.DeepEqual(original.Status, app.Status) {
		return nil
	}

	// The projection reads the conditions already on the resource (Installed is sticky), so a patch
	// computed from a stale cached copy must fail rather than replace the conditions: the optimistic
	// lock turns it into a conflict, and the name is requeued.
	patch := client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{})
	if err := s.client.Status().Patch(ctx, app, patch); err != nil {
		return fmt.Errorf("patch application status: %w", err)
	}

	return nil
}

func (s *Service) computeAndApplyConditions(ev string, app *v1alpha1.Application) {
	report, known := s.source.Report(ev)
	obs := s.source.Observed(ev)

	if app.Status.CurrentVersion == nil {
		app.Status.CurrentVersion = new(v1alpha1.ApplicationStatusVersion)
	}

	// The CR is the authority on deletion: the engine forgets a removed package, and never
	// knows one after a restart mid-deletion.
	deleting := !app.DeletionTimestamp.IsZero()

	f := buildFacts(input{
		Report:         report,
		Known:          known,
		Observation:    obs,
		External:       app.Status.Conditions,
		CurrentVersion: app.Status.CurrentVersion.Version,
		Deleting:       deleting,
	})

	m := mapConditions(&f)
	for _, cond := range m.set {
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

	// Gated conditions are not reported while their gate is closed: until the
	// application is installed, Installed is the only condition it carries.
	for _, condType := range m.remove {
		meta.RemoveStatusCondition(&app.Status.Conditions, condType)
	}

	// Nothing about a live install is committed onto a resource on its way out.
	if !deleting && known {
		// The activated version: Boot restores it, so it is written at activation.
		if report.CurrentVersion != "" {
			app.Status.CurrentVersion.Version = report.CurrentVersion
		}

		// The endpoints of the release the running version applied.
		if f.manifests.isTrue() {
			app.Status.URLs = nil
			for _, url := range obs.URLs {
				app.Status.URLs = append(app.Status.URLs, v1alpha1.ApplicationStatusURL{
					URL:         url.URL,
					Description: url.Description,
				})
			}
		}

		// The settings checkpoint Boot restores, as effective values.
		if values := report.Current.AppliedValues; values != nil {
			if raw, err := json.Marshal(values); err == nil {
				app.Status.LastAppliedConfiguration = runtime.RawExtension{Raw: raw}
			}
		}
	}

	// An empty progress leaves the previous tracking on the CR.
	if !obs.Tracking.Empty() {
		if raw, err := json.Marshal(obs.Tracking); err == nil {
			app.Status.Tracking = runtime.RawExtension{Raw: raw}
		}
	}

	// Summary is computed from the same facts the mappers read.
	summary := summarize(&f)
	app.Status.Summary = &v1alpha1.ApplicationStatusSummary{
		State:   summary.state,
		Message: summary.message,
		Tip:     summary.tip,
	}
}
