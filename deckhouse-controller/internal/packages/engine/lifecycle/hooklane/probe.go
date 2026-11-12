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

// The readiness probe: the run of a package's readiness hook.

package hooklane

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/flant/shell-operator/pkg/executor"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
)

// sdkErrorPrefix starts the message a module-sdk hook prints in its error envelope when its function
// failed; what follows is the function's own error.
const sdkErrorPrefix = "execute: hook reconcile func: "

// probe runs the readiness hook: the outcome of the run is the probe's verdict, which the
// observations record. It never fails the item: a package that is not ready must not hold the
// hooks queued behind its probe, and the schedule runs the probe again.
func (l *Lanes) probe(ctx context.Context, it *item) {
	err := l.run(ctx, it)
	if ctx.Err() != nil {
		// The lane stops: the run was cut short, and the stop resets the verdict.
		return
	}

	verdict := observed.Probe{Ready: err == nil}
	if err != nil {
		verdict.Message = probeMessage(err)
		l.logger.Debug("readiness probe failed", slog.String("hook", it.hook), slog.String("message", verdict.Message))
	}

	l.router.failures.RecordProbe(l.name, verdict)
}

// probeMessage is why the probe found the package not ready: the IsReady message of an SDK that
// reports through the Module, or the error a module-sdk hook printed in its error envelope; any
// other failure as it is.
func probeMessage(err error) string {
	var failed *hooks.ProbeFailedError
	if errors.As(err, &failed) {
		return failed.Message
	}

	var stderr *executor.StderrError
	if errors.As(err, &stderr) {
		var envelope struct {
			Message string `json:"message"`
		}

		if json.Unmarshal([]byte(stderr.Message), &envelope) == nil && envelope.Message != "" {
			return strings.TrimPrefix(envelope.Message, sdkErrorPrefix)
		}
	}

	return err.Error()
}
