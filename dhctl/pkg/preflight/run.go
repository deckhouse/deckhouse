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

package preflightnew

import (
	"context"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// RunSuite runs one suite under its own title, with no cache.
//
// It is for the commands that are not bootstrap: converge, destroy and check reach the same
// machines over the same SSH, and had no preflight at all. An operator with a key the node does
// not accept waited about four minutes of "Try to connect to host" before "Timeout while
// \"Waiting for SSH connection\""; one without sudo waited for "Timeout while \"Get Kubernetes
// API client\"". Both are one SSH round trip to establish.
//
// No cache: these commands run against a cluster that already exists and changes between runs,
// and the question being asked is whether the machines answer now.
func RunSuite(ctx context.Context, suite Suite, phase Phase, title string, preflightOpts *options.PreflightOptions) error {
	if suite == nil || len(suite.Checks()) == 0 {
		return nil
	}

	preflight := New(suite)
	preflight.SetTitle(title)
	if preflightOpts != nil {
		preflight.DisableChecks(preflightOpts.DisabledChecks()...)
		preflight.SetSkippedAll(preflightOpts.SkipAll)
		preflight.SetFailFast(preflightOpts.FailFast)
	}
	return preflight.Run(ctx, phase)
}
