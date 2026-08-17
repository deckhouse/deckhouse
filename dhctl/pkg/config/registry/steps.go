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
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/kubeerrors"
)

const (
	secretsNamespace = "d8-system"
	stateSecretName  = "registry-state"
	initSecretName   = "registry-init"

	conditionTypeReady = "Ready"
)

// errRegistryCheckTransient marks a transport/API-level failure while reading or deleting the
// registry init/state secrets, as opposed to a permanent parse failure or authorization error
// that will fail identically on every attempt.
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
// There are exactly two things this can wait on, and which one applies is decided from the
// configuration rather than from what happens to be present in the cluster.
//
//   - A store the module runs. Its own RegistryStorage status is the answer.
//   - The previous implementation's state machine. Its `registry-state` secret is the answer.
//
// The second one is reachable only on a cluster the previous implementation still owns, and that is
// never a cluster being installed now: the current implementation takes over from the start, and it
// clears the values that secret is written from. Waiting on it there is waiting for something nobody
// will write — a hundred attempts, thirty-three minutes, and then a failed installation of a cluster
// that was working.
//
// Everything else is not waited on at all, and that is a statement rather than a gap: a cluster that
// pulls straight from a registry has nothing in it that reports on that registry, and the moment
// Deckhouse is running is the moment its pull path is known to work.
//
// Parameters:
//   - ctx: context for cancellation and timeouts
//   - kubeClient: Kubernetes client for API operations
//   - config: configuration with registry settings
//
// Returns:
//   - err: error from the operation
func isRegistryReady(ctx context.Context, kubeClient client.KubeClient, config Config) error {
	logger := dhlog.FromContext(ctx)

	if config.StoreExpected {
		// Fullness is required only when the store is the only source of images. With an upstream
		// configured the cluster can pull through the store while it is still filling, so demanding a
		// complete copy would add the whole first sync — gigabytes, over the operator's link — to
		// every installation that turns the cache on. Without an upstream there is nowhere else to
		// pull from, and a store that is merely Ready is a store that will answer "no such host" for
		// an image it has not copied yet.
		if err := isStoreReady(ctx, kubeClient, config.BundleBootstrap); err != nil {
			logger.DebugContext(ctx, fmt.Sprintf("Error while checking the cluster store: %v", err))
			return err
		}
		return nil
	}

	if !isLegacyImplementation(config) {
		logger.DebugContext(ctx,
			"No in-cluster registry is expected for this configuration, so there is nothing to wait for")
		return nil
	}

	conditions, err := getConditions(ctx, kubeClient)
	if err != nil {
		logger.DebugContext(ctx, fmt.Sprintf("Error while checking registry ready: %v", err))
		return ErrIsNotReady
	}

	if isConditionsReady(conditions) {
		return nil
	}

	if msg := formatNotReadyMessage(conditions); msg != "" {
		err := fmt.Errorf("%s\n%s", ErrIsNotReady.Error(), msg)
		logger.DebugContext(ctx, fmt.Sprintf("Error while checking registry ready: %v", err))
		return err
	}

	return ErrIsNotReady
}

// isLegacyImplementation reports that the configuration asks for something only the previous
// implementation provides, and so that its state machine is what reports readiness.
//
// Proxy and Local are the two modes whose whole content is a registry the previous implementation
// runs in the cluster — its pull-through proxy, or its local store. Direct and Unmanaged are not:
// they configure the container runtime to reach a registry that already exists, which is exactly
// what the current implementation's fallback does, and there is nothing in the cluster to wait for.
//
// That distinction is why this is not simply `!LegacyMode`. Direct is the default for a cluster with
// a supported container runtime, so reading it as "the previous implementation reports on this" is
// what made every ordinary installation wait for a secret that is no longer written.
func isLegacyImplementation(config Config) bool {
	switch config.Settings.Mode {
	case constant.ModeProxy, constant.ModeLocal:
		return true
	default:
		return false
	}
}

// formatNotReadyMessage builds a human-readable message listing all non-True
// conditions (excluding the Ready condition itself).
func formatNotReadyMessage(conditions []metav1.Condition) string {
	if len(conditions) == 0 {
		return ""
	}

	var msg strings.Builder

	for _, condition := range conditions {
		if condition.Type == conditionTypeReady {
			continue
		}

		if condition.Status == metav1.ConditionTrue {
			continue
		}

		if msg.Len() > 0 {
			msg.WriteString("\n")
		}

		fmt.Fprintf(&msg, "* %s: %s",
			condition.Type,
			strings.TrimSpace(strings.ReplaceAll(condition.Message, "\n", " ")),
		)
	}

	return msg.String()
}

// isConditionsReady checks whether the registry is ready based on its conditions.
// It returns true only if the Ready condition is present and set to True.
func isConditionsReady(conditions []metav1.Condition) bool {
	if len(conditions) == 0 {
		return false
	}

	for _, condition := range conditions {
		if condition.Type == conditionTypeReady {
			return condition.Status == metav1.ConditionTrue
		}
	}

	return false
}

// getConditions retrieves and parses the registry state conditions.
// Parameters:
//   - ctx: context for cancellation and timeouts
//   - kubeClient: Kubernetes client for API operations
//
// Returns:
//   - []metav1.Condition: registry state conditions
//   - err: error from the operation
func getConditions(ctx context.Context, kubeClient client.KubeClient) ([]metav1.Condition, error) {
	var conditions []metav1.Condition

	secret, err := kubeClient.
		CoreV1().
		Secrets(secretsNamespace).
		Get(ctx, stateSecretName, metav1.GetOptions{})

	if err != nil {
		if apierrors.IsNotFound(err) {
			// No status reported yet: equivalent to no conditions being ready.
			return nil, nil
		}
		if kubeerrors.IsPermanentAuthError(ctx, err) {
			return nil, fmt.Errorf("get secret '%s/%s': %w", secretsNamespace, stateSecretName, err)
		}
		return nil, fmt.Errorf("%w: get secret '%s/%s': %w", errRegistryCheckTransient, secretsNamespace, stateSecretName, err)
	}

	rawConditions, exists := secret.Data["conditions"]
	if !exists {
		return conditions, nil
	}

	if err := yaml.Unmarshal(rawConditions, &conditions); err != nil {
		return nil, fmt.Errorf(
			"unmarshal secret '%s/%s' conditions: %w",
			secretsNamespace, stateSecretName, err,
		)
	}

	return conditions, nil
}
