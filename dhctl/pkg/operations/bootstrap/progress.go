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

package bootstrap

import (
	"context"
	"log/slog"

	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

// runProgress opens a terminal progress session named name, runs body (which
// emits phases.Progress events into the supplied channel), and closes the
// session afterwards. The bar logic lives in pkg/operations/phases so it can be
// shared with converge and the dhctl commands.
func runProgress(ctx context.Context, l *slog.Logger, name string, body func(progressCh chan phases.Progress) error) error {
	return phases.RunProgress(ctx, l, name, body)
}
