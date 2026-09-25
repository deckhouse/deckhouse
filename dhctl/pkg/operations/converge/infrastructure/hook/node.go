// Copyright 2021 Flant JSC
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

package hook

import (
	"context"
	"fmt"
	"time"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
)

var ErrNotReady = fmt.Errorf("Not ready.")

type NodeChecker interface {
	IsReady(ctx context.Context, nodeName string) (bool, error)
	Name() string
}

func IsNodeReady(ctx context.Context, checkers []NodeChecker, nodeName, sourceCommandName string) (bool, error) {
	_ = sourceCommandName
	title := fmt.Sprintf("Node %s readiness check", nodeName)
	var lastErr error

	err := retry.NewLoop(title, 300, 1*time.Second).RunContext(ctx, func() error {
		for _, check := range checkers {
			// No process block per checker. This body is one attempt of a retry loop, and a
			// block opened here opens and fails on every attempt - which the live UI restates
			// as a persistent FAILED milestone each time, so a node that simply took forty
			// seconds to come up left forty identical "FAILED Control plane readiness" rows
			// pinned on screen and forty more in the closing summary. The retry loop already
			// frames itself as one block; the checker's identity belongs in the error, which
			// the loop logs per attempt and carries into its exhaustion message.
			isReady, err := check.IsReady(ctx, nodeName)
			if err != nil {
				lastErr = fmt.Errorf("%s: %w", check.Name(), err)
				return lastErr
			}

			if !isReady {
				lastErr = fmt.Errorf("%s: %w", check.Name(), ErrNotReady)
				return lastErr
			}
		}

		return nil
	})
	if err != nil {
		return false, fmt.Errorf("Node %s is not ready. Last error: %v/%v", nodeName, err, lastErr)
	}

	return true, nil
}

type KubeNodeReadinessChecker struct {
	getter kubernetes.KubeClientProviderWithCtx
}

func NewKubeNodeReadinessChecker(getter kubernetes.KubeClientProviderWithCtx) *KubeNodeReadinessChecker {
	return &KubeNodeReadinessChecker{
		getter: getter,
	}
}

func (c *KubeNodeReadinessChecker) IsReady(ctx context.Context, nodeName string) (bool, error) {
	kubeClient, err := c.getter.KubeClientCtx(ctx)
	if err != nil {
		return false, fmt.Errorf("Could not get kube client: %w", err)
	}

	node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	for _, c := range node.Status.Conditions {
		if c.Type == apiv1.NodeReady {
			if c.Status == apiv1.ConditionTrue {
				return true, nil
			}
		}
	}

	return false, nil
}

func (c *KubeNodeReadinessChecker) Name() string {
	return "Kube node is ready"
}
