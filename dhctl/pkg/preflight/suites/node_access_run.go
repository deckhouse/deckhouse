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

package suites

import (
	"context"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

// RunNodeAccessPreflights checks that dhctl can reach and use the machines an operation is about
// to touch, before it starts touching them.
//
// It lives here, next to the suite, rather than in the command layer, so the gRPC path gets it
// too: Commander has always sent skip_preflight_checks to converge, check and destroy, where
// nothing read them, because those operations had no preflight checks at all to skip.
//
// A no-op where there is no node to look at — a cluster reached over a kubeconfig.
func RunNodeAccessPreflights(
	ctx context.Context,
	sshProviderInitializer *providerinitializer.SSHProviderInitializer,
	preflightOpts *options.PreflightOptions,
	title string,
) error {
	if sshProviderInitializer == nil || !sshProviderInitializer.CheckHosts(ctx) {
		dhlog.FromContext(ctx).DebugContext(ctx, "No SSH hosts known: skipping the node preflight checks")
		return nil
	}

	suite := NewNodeAccessSuite(NodeAccessDeps{SSHProviderInitializer: sshProviderInitializer})
	return preflight.RunSuite(ctx, suite, preflight.PhasePostInfra, title, preflightOpts)
}
