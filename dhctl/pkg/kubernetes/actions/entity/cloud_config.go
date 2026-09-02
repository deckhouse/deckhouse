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

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

const (
	cloudConfigSecretNamespace = "d8-cloud-instance-manager"
	cloudConfigWaitTimeout     = 225 * time.Second
)

type cloudConfigSecretState struct {
	cloudConfig     string
	endpoints       []string
	missingHosts    []string
	resourceVersion string
}

func inspectCloudConfigSecret(
	secret *corev1.Secret,
	expectedHosts []string,
) (cloudConfigSecretState, error) {
	if secret == nil {
		return cloudConfigSecretState{}, errors.New("cloud config secret is nil")
	}

	state := cloudConfigSecretState{
		resourceVersion: secret.ResourceVersion,
	}

	if len(expectedHosts) > 0 {
		endpointsRaw := secret.Data["apiserverEndpoints"]
		if len(endpointsRaw) == 0 {
			state.missingHosts = slices.Sorted(
				maps.Keys(hostSet(expectedHosts)),
			)

			return state, errors.New(
				"apiserverEndpoints is missing or empty",
			)
		}

		if err := yaml.Unmarshal(endpointsRaw, &state.endpoints); err != nil {
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
			if strings.TrimSpace(expectedHost) == "" {
				return state, errors.New(
					"expected API server host is empty",
				)
			}

			if _, ok := currentHosts[expectedHost]; !ok {
				missingHosts[expectedHost] = struct{}{}
			}
		}

		state.missingHosts = slices.Sorted(maps.Keys(missingHosts))
		if len(state.missingHosts) > 0 {
			return state, fmt.Errorf(
				"API server hosts not found in cloud config: %s",
				strings.Join(state.missingHosts, ", "),
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

func hostSet(hosts []string) map[string]struct{} {
	result := make(map[string]struct{}, len(hosts))

	for _, host := range hosts {
		if host == "" {
			continue
		}

		result[host] = struct{}{}
	}

	return result
}

func waitForCloudConfigSecret(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	nodeGroupName string,
	expectedHosts []string,
	timeout time.Duration,
) (cloudConfigSecretState, error) {
	secretName := "manual-bootstrap-for-" + nodeGroupName

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	selector := fields.OneTermEqualSelector(
		"metadata.name",
		secretName,
	).String()

	secrets := kubeCl.CoreV1().Secrets(cloudConfigSecretNamespace)

	var (
		lastState                 cloudConfigSecretState
		lastErr                   error
		lastLoggedResourceVersion string
		hasLoggedState            bool
	)

	recordState := func(secret *corev1.Secret) bool {
		state, inspectErr := inspectCloudConfigSecret(
			secret,
			expectedHosts,
		)

		if !hasLoggedState ||
			state.resourceVersion != lastLoggedResourceVersion {
			dhlog.FromContext(ctx).DebugContext(
				ctx,
				fmt.Sprintf(
					"Cloud config Secret %s/%s changed: "+
						"resourceVersion=%q, endpoints=%v, "+
						"missing hosts=%v, validation error=%v",
					cloudConfigSecretNamespace,
					secretName,
					state.resourceVersion,
					state.endpoints,
					state.missingHosts,
					inspectErr,
				),
			)

			lastLoggedResourceVersion = state.resourceVersion
			hasLoggedState = true
		}

		lastState = state
		lastErr = inspectErr

		return inspectErr == nil
	}

	waitError := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}

		return fmt.Errorf(
			"timeout after %s waiting for cloud config Secret %s/%s: "+
				"resourceVersion=%q, current endpoints=%v, "+
				"missing hosts=%v, last error: %v",
			timeout,
			cloudConfigSecretNamespace,
			secretName,
			lastState.resourceVersion,
			lastState.endpoints,
			lastState.missingHosts,
			lastErr,
		)
	}

	waitBeforeRetry := func() bool {
		timer := time.NewTimer(time.Second)
		defer timer.Stop()

		select {
		case <-waitCtx.Done():
			return false
		case <-timer.C:
			return true
		}
	}

	for {
		if waitCtx.Err() != nil {
			return lastState, waitError()
		}

		secretList, err := secrets.List(
			waitCtx,
			metav1.ListOptions{
				FieldSelector: selector,
			},
		)
		if err != nil {
			lastErr = fmt.Errorf(
				"list cloud config Secret %s/%s: %w",
				cloudConfigSecretNamespace,
				secretName,
				err,
			)

			if !waitBeforeRetry() {
				return lastState, waitError()
			}

			continue
		}

		for i := range secretList.Items {
			if recordState(&secretList.Items[i]) {
				return lastState, nil
			}
		}

		if len(secretList.Items) == 0 {
			lastErr = fmt.Errorf(
				"cloud config Secret %s/%s not found",
				cloudConfigSecretNamespace,
				secretName,
			)
		}

		watcher, err := secrets.Watch(
			waitCtx,
			metav1.ListOptions{
				FieldSelector:       selector,
				ResourceVersion:     secretList.ResourceVersion,
				AllowWatchBookmarks: true,
			},
		)
		if err != nil {
			lastErr = fmt.Errorf(
				"start watch for cloud config Secret %s/%s: %w",
				cloudConfigSecretNamespace,
				secretName,
				err,
			)

			if !waitBeforeRetry() {
				return lastState, waitError()
			}

			continue
		}

		watchClosed := false

		for !watchClosed {
			select {
			case <-waitCtx.Done():
				watcher.Stop()
				return lastState, waitError()

			case event, ok := <-watcher.ResultChan():
				if !ok {
					lastErr = fmt.Errorf(
						"watch for cloud config Secret %s/%s was closed",
						cloudConfigSecretNamespace,
						secretName,
					)
					watchClosed = true
					continue
				}

				switch event.Type {
				case watch.Added, watch.Modified:
					secret, ok := event.Object.(*corev1.Secret)
					if !ok {
						lastErr = fmt.Errorf(
							"unexpected object type %T",
							event.Object,
						)
						continue
					}

					if recordState(secret) {
						watcher.Stop()
						return lastState, nil
					}

				case watch.Deleted:
					lastErr = fmt.Errorf(
						"cloud config Secret %s/%s was deleted",
						cloudConfigSecretNamespace,
						secretName,
					)

				case watch.Error:
					lastErr = apierrors.FromObject(event.Object)
					watchClosed = true

				case watch.Bookmark:
					// The bookmark only advances resourceVersion.
					// No Secret data needs to be inspected.
				}
			}
		}

		watcher.Stop()

		if !waitBeforeRetry() {
			return lastState, waitError()
		}
	}
}
