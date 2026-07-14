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

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

const (
	secretsNamespace            = "d8-system"
	stateSecretName             = "registry-state"
	initSecretName              = "registry-init"
	initSecretAppliedAnnotation = "registry.deckhouse.io/is-applied"

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
func WaitForRegistryInitialization(ctx context.Context, kubeClient client.KubeClient, config Config) error {
	loopParams := retry.NewEmptyParams(
		retry.WithName("Waiting for Registry to become Ready"),
		retry.WithAttempts(100),
		retry.WithWait(20*time.Second),
		// ErrIsNotReady/ErrNotInitialized are the expected "still becoming ready" conditions;
		// errRegistryCheckTransient covers transport-level hiccups. Anything else (a malformed
		// secret, a permission failure) is permanent and should stop the loop immediately.
		retry.WithWhitelist(ErrIsNotReady, ErrNotInitialized, errRegistryCheckTransient),
	)

	return retry.NewLoopWithParams(loopParams).
		RunContext(ctx, func() error {
			return checkRegistryInitialization(ctx, kubeClient, config)
		})
}

// checkRegistryInitialization performs checks for registry initialization status.
// After successful initialization, the initSecret will be removed.
// Parameters:
//   - ctx: context for cancellation and timeouts
//   - kubeClient: Kubernetes client for API operations
//   - config: configuration with registry settings
//
// Returns:
//   - err: error from the operation
func checkRegistryInitialization(ctx context.Context, kubeClient client.KubeClient, config Config) error {
	if !config.LegacyMode {
		if err := checkInit(ctx, kubeClient); err != nil {
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Error while checking registry init: %v", err))
			return err
		}

		msg, err := checkReady(ctx, kubeClient)
		if err != nil {
			if msg != "" {
				wrapped := fmt.Errorf("%w\n%s", err, msg)
				dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Error while checking registry ready: %v", wrapped))
				return wrapped
			}

			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Error while checking registry ready: %v", err))
			return err
		}
	}

	if err := removeInitSecret(ctx, kubeClient); err != nil {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Error while removing registry init secret: %v", err))
		return err
	}

	return nil
}

// checkInit verifies if the registry initialization process has started.
// Parameters:
//   - ctx: context for cancellation and timeouts
//   - kubeClient: Kubernetes client for API operations
//
// Returns:
//   - err: error from the operation
func checkInit(ctx context.Context, kubeClient client.KubeClient) error {
	exists, applied, err := getInitSecretStatus(ctx, kubeClient)
	if err != nil {
		return err
	}

	if exists && !applied {
		return ErrNotInitialized
	}
	return nil
}

// checkReady verifies if the registry is ready.
// Parameters:
//   - ctx: context for cancellation and timeouts
//   - kubeClient: Kubernetes client for API operations
//
// Returns:
//   - string: readiness status messages
//   - err: error from the operation
func checkReady(ctx context.Context, kubeClient client.KubeClient) (string, error) {
	conditions, err := getStateSecret(ctx, kubeClient)
	if err != nil {
		return "", err
	}

	if len(conditions) == 0 {
		return "", ErrIsNotReady
	}

	var (
		msg   strings.Builder
		ready bool
	)

	for _, condition := range conditions {
		if condition.Type == conditionTypeReady {
			ready = condition.Status == metav1.ConditionTrue
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

	if ready {
		return "", nil
	}

	return msg.String(), ErrIsNotReady
}

// getStateSecret retrieves and parses the registry state conditions.
// Parameters:
//   - ctx: context for cancellation and timeouts
//   - kubeClient: Kubernetes client for API operations
//
// Returns:
//   - []metav1.Condition: registry state conditions
//   - err: error from the operation
func getStateSecret(ctx context.Context, kubeClient client.KubeClient) ([]metav1.Condition, error) {
	secret, err := kubeClient.
		CoreV1().
		Secrets(secretsNamespace).
		Get(ctx, stateSecretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// No status reported yet: equivalent to no conditions being ready.
			return nil, nil
		}
		if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) {
			return nil, fmt.Errorf("get secret '%s/%s': %w", secretsNamespace, stateSecretName, err)
		}
		return nil, fmt.Errorf("%w: get secret '%s/%s': %w", errRegistryCheckTransient, secretsNamespace, stateSecretName, err)
	}

	var conditions []metav1.Condition

	conditionRaw, exists := secret.Data["conditions"]
	if !exists {
		return conditions, nil
	}

	if err := yaml.Unmarshal(conditionRaw, &conditions); err != nil {
		return nil, fmt.Errorf("unmarshal secret data: %w", err)
	}

	return conditions, nil
}

// getInitSecretStatus checks the status of the init secret.
// Parameters:
//   - ctx: context for cancellation and timeouts
//   - kubeClient: Kubernetes client for API operations
//
// Returns:
//   - secretExists: boolean indicating secret presence
//   - secretApplied: boolean indicating secret application status
//   - err: error from the operation
func getInitSecretStatus(ctx context.Context, kubeClient client.KubeClient) (bool, bool, error) {
	secret, err := kubeClient.
		CoreV1().
		Secrets(secretsNamespace).
		Get(ctx, initSecretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, false, nil
		}
		if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) {
			return false, false, fmt.Errorf("get secret '%s/%s': %w", secretsNamespace, initSecretName, err)
		}
		return false, false, fmt.Errorf("%w: get secret '%s/%s': %w", errRegistryCheckTransient, secretsNamespace, initSecretName, err)
	}

	_, applied := secret.Annotations[initSecretAppliedAnnotation]
	return true, applied, nil
}

// removeInitSecret removes the initialization secret.
// Parameters:
//   - ctx: context for cancellation and timeouts
//   - kubeClient: Kubernetes client for API operations
//
// Returns:
//   - err: error from the operation
func removeInitSecret(ctx context.Context, kubeClient client.KubeClient) error {
	err := kubeClient.
		CoreV1().
		Secrets(secretsNamespace).
		Delete(ctx, initSecretName, metav1.DeleteOptions{})

	if err != nil && !apierrors.IsNotFound(err) {
		if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) {
			return fmt.Errorf("remove secret '%s/%s': %w", secretsNamespace, initSecretName, err)
		}
		return fmt.Errorf("%w: remove secret '%s/%s': %w", errRegistryCheckTransient, secretsNamespace, initSecretName, err)
	}

	return nil
}
