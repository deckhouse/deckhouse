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

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

var (
	ErrNotReady = fmt.Errorf("Not ready.")
)

type NodeChecker interface {
	IsReady(ctx context.Context, nodeName string) (bool, error)
	Name() string
}

func IsNodeReady(ctx context.Context, checkers []NodeChecker, nodeName, sourceCommandName string) (bool, error) {
	title := fmt.Sprintf("Node %s readiness check", nodeName)
	var lastErr error

	err := retry.NewLoop(title, 30, 10*time.Second).RunContext(ctx, func() error {
		for _, check := range checkers {
			err := log.Process(sourceCommandName, check.Name(), func() error {
				isReady, err := check.IsReady(ctx, nodeName)
				if err != nil {
					return err
				}

				if !isReady {
					return ErrNotReady
				}

				return err
			})

			if err != nil {
				lastErr = err
				return err
			}
		}

		return nil
	})

	if err != nil {
		return false, fmt.Errorf("Node %s is not ready. last error: %v/%v", nodeName, err, lastErr)
	}

	return true, nil
}

type KubeNodeReadinessChecker struct {
	getter kubernetes.KubeClientProvider
}

func NewKubeNodeReadinessChecker(getter kubernetes.KubeClientProvider) *KubeNodeReadinessChecker {
	return &KubeNodeReadinessChecker{
		getter: getter,
	}
}

func (c *KubeNodeReadinessChecker) IsReady(ctx context.Context, nodeName string) (bool, error) {
	node, err := c.getter.KubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
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
