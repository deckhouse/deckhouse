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

package current

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/configstate"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/resourcesize"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// accept takes config into the object, only while the worker is idle (configstate.State.Accept):
// a failure keeps the last accepted configuration in force and is retried with backoff
// (validation may read the cluster and fail transiently); Configured reports the outcome.
func (r *Reconciler) accept(ctx context.Context, config *model.Configuration, checksum string) {
	settings := &r.state.settings

	var values addonutils.Values

	out := settings.Accept(checksum, &r.state.accept, r.clock, func() (addonutils.Values, string, error) {
		if err := r.configure(ctx, config); err != nil {
			return nil, "", err
		}

		values = r.pkg.GetSettings()
		sum, err := valuesChecksum(values, config.Maintenance, config.ResourceRequests)

		return values, sum, err
	})

	if out.Err != nil {
		if ctx.Err() != nil {
			// Stopping: nothing is reported, the next Current accepts it. The backoff only keeps
			// the loop from trying again before it sees the cancellation.
			return
		}

		r.reporter.UpdateCurrent(func(r *model.CurrentReport) {
			r.Configured = model.FalseCondition(model.ReasonSettingsInvalid, out.Err.Error())
		})

		r.logger.Warn("configuration not accepted, the last accepted one stays in force",
			slog.Duration("retry_in", out.Until.Sub(r.clock.Now())), log.Err(out.Err))

		return
	}

	// A disabled package has nothing more to apply: the accepted configuration is its checkpoint,
	// if it has run (hasRun).
	cp := out.Changed && r.state.disabled
	accepted := settings.Accepted

	// Reported on every successful accept, before and independently of a run (AcceptedValues).
	r.reporter.UpdateCurrent(func(cr *model.CurrentReport) {
		cr.Configured = model.TrueCondition()
		cr.AcceptedValues, cr.AcceptedChecksum = values, accepted.Checksum
		cr.Maintenance = config.Maintenance

		if cp && r.hasRun(cr) {
			accepted.Checkpoint(cr)
		}
	})
}

// configure: a failure leaves the previous settings in force (ValidateSettings changes nothing, a
// failed ApplySettings stores nothing).
func (r *Reconciler) configure(ctx context.Context, config *model.Configuration) error {
	settings := config.Settings
	version := settings.SchemaVersion

	res, err := r.pkg.ValidateSettings(ctx, version, settings.Values)
	if err != nil {
		return fmt.Errorf("validate settings: %w", err)
	}

	if !res.Valid {
		return fmt.Errorf("validate settings: %s", res.Message)
	}

	if err := r.pkg.ApplySettings(version, settings.Values); err != nil {
		return fmt.Errorf("apply settings: %w", err)
	}

	r.pkg.SetMaintenance(config.Maintenance)

	if sizer, ok := r.pkg.(resourceSizer); ok {
		sizer.SetResourceRequests(config.ResourceRequests)
	}

	return nil
}

// valuesChecksum is the checksum of the effective values the object holds, with the maintenance
// mode and the resource requests (SHA-256 of their JSON, sorted keys); it reaches the Report as
// AcceptedChecksum and AppliedChecksum. The effective values are already in the revision's schema,
// so the schema version is not part of it.
func valuesChecksum(values addonutils.Values, maintenance nelm.MaintenanceState, requests []resourcesize.Request) (string, error) {
	return configstate.Checksum(struct {
		Settings         addonutils.Values      `json:"settings"`
		Maintenance      nelm.MaintenanceState  `json:"maintenance"`
		ResourceRequests []resourcesize.Request `json:"resourceRequests"`
	}{values, maintenance, requests})
}
