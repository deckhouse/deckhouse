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

package registry

// Air-gap (NeedsSeed) bootstrap cache finalize: wait for the module cache + agent
// DaemonSets to take over, then delete the temporary bootstrap cache (brought up
// earlier in the bootstrapRegistry phase, see bootstrap_cache.go).

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

const (
	cacheReadyPollAttempts = 150 // ×10s = 25 min
	cacheReadyPollWait     = 10 * time.Second

	bootstrapCacheNamespace = "d8-system"
)

// Torn down at finalize. The fill Pod/secret are normally deleted inline on
// success; listed here for resume/crash safety.
var (
	bootstrapCachePods = []string{
		bootstrapCachePodName,
		bootstrapCacheFillPodName,
	}
	bootstrapCacheSecrets = []string{
		bootstrapCachePKISecret,
		bootstrapCacheConfigSecret,
		bootstrapCacheFillSecret,
	}
)

// WaitForCacheAndAgentReady waits for the registry-cache and registry-agent
// DaemonSets to have all desired pods Ready.
func WaitForCacheAndAgentReady(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Waiting for registry-cache + registry-agent to become Ready", cacheReadyPollAttempts, cacheReadyPollWait).
		RunContext(ctx, func() error {
			return checkCacheAndAgentReady(ctx, kubeCl)
		})
}

func checkCacheAndAgentReady(ctx context.Context, kubeCl *client.KubernetesClient) error {
	for _, name := range []string{"registry-cache", "registry-agent"} {
		ds, err := kubeCl.AppsV1().DaemonSets("d8-system").Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get daemonset %s: %w", name, err)
		}
		if ds.Status.DesiredNumberScheduled < 1 || ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
			return fmt.Errorf("%s not ready (desired=%d ready=%d)", name, ds.Status.DesiredNumberScheduled, ds.Status.NumberReady)
		}
	}

	return nil
}

// DeleteBootstrapCache removes the temporary bootstrap cache pods + secrets; the
// hostPath data is left for the module cache pod. Idempotent (NotFound tolerated).
func DeleteBootstrapCache(ctx context.Context, kubeCl *client.KubernetesClient) error {
	for _, name := range bootstrapCachePods {
		if err := kubeCl.CoreV1().Pods(bootstrapCacheNamespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete pod %s: %w", name, err)
		}
	}

	for _, name := range bootstrapCacheSecrets {
		if err := kubeCl.CoreV1().Secrets(bootstrapCacheNamespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			// Log-only: a leftover secret is harmless; don't fail the bootstrap over it.
			log.GetDefaultLogger().LogWarnLn("delete bootstrap cache secret", name, ":", err)
		}
	}
	return nil
}
