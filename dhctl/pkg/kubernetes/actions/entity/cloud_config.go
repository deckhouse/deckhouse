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

package entity

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"maps"
	"net"
	"slices"
	"strings"
	"time"

	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/retry"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/kubeerrors"
)

const cloudConfigSecretNamespace = "d8-cloud-instance-manager"

type cloudConfigSecretState struct {
	cloudConfig     string
	endpoints       []string
	resourceVersion string
}

func inspectCloudConfigSecret(
	secret *corev1.Secret,
	expectedHosts []string,
) (cloudConfigSecretState, error) {
	state := cloudConfigSecretState{
		resourceVersion: secret.ResourceVersion,
	}

	if len(expectedHosts) > 0 {
		if err := yaml.Unmarshal(secret.Data["apiserverEndpoints"], &state.endpoints); err != nil {
			return state, fmt.Errorf(
				"unmarshal apiserverEndpoints: %w",
				err,
			)
		}

		currentHosts := make(map[string]struct{}, len(state.endpoints))
		for _, endpoint := range state.endpoints {
			host, _, err := net.SplitHostPort(endpoint)
			if err != nil {
				return state, fmt.Errorf(
					"split API server endpoint %q: %w",
					endpoint,
					err,
				)
			}

			currentHosts[host] = struct{}{}
		}

		missingHosts := make(map[string]struct{})
		for _, expectedHost := range expectedHosts {
			if _, ok := currentHosts[expectedHost]; !ok {
				missingHosts[expectedHost] = struct{}{}
			}
		}

		if len(missingHosts) > 0 {
			return state, fmt.Errorf(
				"API server hosts not found in apiserverEndpoints: %s",
				strings.Join(slices.Sorted(maps.Keys(missingHosts)), ", "),
			)
		}
	}

	cloudConfigRaw := secret.Data["cloud-config"]
	if len(cloudConfigRaw) == 0 {
		return state, errors.New("cloud-config is missing or empty")
	}

	state.cloudConfig = base64.StdEncoding.EncodeToString(cloudConfigRaw)

	return state, nil
}

// waitForCloudConfigSecret polls the manual-bootstrap-for-<nodeGroupName> Secret until it
// carries a cloud-config and, when expectedHosts is set, lists all of them in apiserverEndpoints.
func waitForCloudConfigSecret(
	ctx context.Context,
	kubeProvider kubernetes.KubeClientProviderWithCtx,
	nodeGroupName string,
	expectedHosts []string,
	attempts int,
	wait time.Duration,
) (string, error) {
	if slices.ContainsFunc(expectedHosts, func(h string) bool { return strings.TrimSpace(h) == "" }) {
		return "", errors.New("expected API server host is empty")
	}

	secretName := "manual-bootstrap-for-" + nodeGroupName
	name := fmt.Sprintf("Wait for cloud config Secret %s/%s", cloudConfigSecretNamespace, secretName)

	var cloudConfig string

	err := retry.NewSilentLoop(name, attempts, wait).
		BreakIf(func(err error) bool { return kubeerrors.IsPermanentAuthError(ctx, err) }).
		RunContext(ctx, func() error {
			kubeCl, err := kubeProvider.KubeClientCtx(ctx)
			if err != nil {
				return err
			}

			secret, err := kubeCl.CoreV1().
				Secrets(cloudConfigSecretNamespace).
				Get(ctx, secretName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			state, err := inspectCloudConfigSecret(secret, expectedHosts)
			if err != nil {
				return fmt.Errorf(
					"resourceVersion=%q, endpoints=%v: %w",
					state.resourceVersion,
					state.endpoints,
					err,
				)
			}

			cloudConfig = state.cloudConfig

			return nil
		})

	return cloudConfig, err
}
