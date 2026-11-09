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

package registry

import (
	"context"
	"fmt"
	"time"

	dhlog "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

const (
	secretsNamespace = "d8-system"
	initSecretName   = "registry-init"

	// initSecretAppliedAnnotation is the previous implementation's mark of a consumed init secret,
	// read for presence by modules/038-registry/hooks/orchestrator/init-secret.
	initSecretAppliedAnnotation = "registry.deckhouse.io/is-applied"
)

// errRegistryCheckTransient marks a transport/API-level failure while reading the registry module's
// RegistryStorage, or its absence before the module has started, as opposed to a permanent parse
// failure or authorization error that will fail identically on every attempt.
var errRegistryCheckTransient = fmt.Errorf("registry check: transient error, may succeed on retry")

// WaitForRegistryInitialization waits for the registry to become fully initialized and ready.
// After successful initialization, the initSecret will be removed.
// Parameters:
//   - ctx: context for cancellation and timeouts
//   - kubeClient: Kubernetes client for API operations
//   - config: configuration with registry settings
//
// Returns:
//   - err: error from the operation
func WaitForRegistryReady(ctx context.Context, kubeClient client.KubeClient, config Config) error {
	return retry.
		NewLoop("Waiting for Registry to become Ready", 100, 20*time.Second).
		RunContext(ctx, func() error {
			return isRegistryReady(ctx, kubeClient, config)
		})
}

// isRegistryReady checks whether the registry the cluster was configured with has become usable.
//
// There is exactly one thing this can wait on, and whether it applies is decided from the
// configuration rather than from what happens to be present in the cluster: a store the module runs,
// answered by its own RegistryStorage status.
//
// The previous implementation's state machine is not waited on any more. Its modes are refused when
// the configuration is parsed (see RefusePreviousImplementationModes), and nothing in a cluster
// installed from this release writes its `registry-state` secret, so a wait for it could only end in
// a timeout.
//
// Everything else is not waited on, and that is a statement rather than a gap: a cluster pulling
// straight from a registry has nothing in it that reports on that registry, and the moment Deckhouse
// is running is the moment its pull path is known to work.
func isRegistryReady(ctx context.Context, kubeClient client.KubeClient, config Config) error {
	logger := dhlog.FromContext(ctx)

	if config.StoreExpected {
		// The store is waited for only where it is the ONLY source of images, which is an
		// installation from a bundle. With an upstream the agent falls back to it for whatever the
		// cache has not copied yet, so the filling is an optimisation that finishes after the
		// installation rather than a precondition for it.
		//
		// Waiting anyway is not merely slow, it fails installations: `Ready` is reported only once
		// the LEADER IS FULL, so the wait becomes a wait for the whole first sync — the entire image
		// set over the operator's link — which the bootstrap watchdog cuts short.
		//
		// Without an upstream nothing here may be relaxed: the cache is where every image comes
		// from, and a store that is merely running answers "no such host" for what it has not
		// copied.
		if !config.BundleBootstrap {
			logger.InfoContext(ctx,
				"The cluster pulls through its upstream, so the cache may finish filling after the "+
					"installation and is not waited for")
			return nil
		}

		if err := isStoreReady(ctx, kubeClient, true); err != nil {
			logger.DebugContext(ctx, fmt.Sprintf("Error while checking the cluster store: %v", err))
			return err
		}
		return nil
	}

	logger.DebugContext(ctx,
		"No in-cluster registry is expected for this configuration, so there is nothing to wait for")
	return nil
}
