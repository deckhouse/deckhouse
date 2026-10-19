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

// Configuration state: pushed global settings validated and applied to the global module.

package global

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/configstate"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// accept is Current's accept (current/settings.go) for the global settings; their checksum
// covers the effective values alone.
func (g *Global) accept(ctx context.Context, settings *model.Settings, checksum string) {
	var values addonutils.Values

	out := g.state.settings.Accept(checksum, &g.state.accept, g.clock, func() (addonutils.Values, string, error) {
		if err := g.configure(ctx, settings); err != nil {
			return nil, "", err
		}

		values = g.module.GetSettings()
		sum, err := configstate.Checksum(values)

		return values, sum, err
	})

	if out.Err != nil {
		if ctx.Err() != nil {
			return
		}

		g.reporter.UpdateCurrent(func(r *model.CurrentReport) {
			r.Configured = model.FalseCondition(model.ReasonSettingsInvalid, out.Err.Error())
		})

		g.logger.Warn("global settings not accepted, the last accepted ones stay in force",
			slog.Duration("retry_in", out.Until.Sub(g.clock.Now())), log.Err(out.Err))

		return
	}

	sum := g.state.settings.Accepted.Checksum

	// Reported before any run succeeds (CurrentReport.AcceptedValues).
	g.reporter.UpdateCurrent(func(r *model.CurrentReport) {
		r.Configured = model.TrueCondition()
		r.AcceptedValues, r.AcceptedChecksum = values, sum
	})
}

func (g *Global) configure(ctx context.Context, settings *model.Settings) error {
	version := settings.SchemaVersion

	res, err := g.module.ValidateSettings(ctx, version, settings.Values)
	if err != nil {
		return fmt.Errorf("validate settings: %w", err)
	}

	if !res.Valid {
		return fmt.Errorf("validate settings: %s", res.Message)
	}

	if err := g.module.ApplySettings(version, settings.Values); err != nil {
		return fmt.Errorf("apply settings: %w", err)
	}

	return nil
}
