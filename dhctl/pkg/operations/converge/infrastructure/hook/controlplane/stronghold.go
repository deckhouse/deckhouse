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

package controlplane

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const (
	strongholdNamespace   = "d8-stronghold"
	strongholdStatefulSet = "stronghold"
)

type StrongholdReadinessChecker struct {
	getter kubernetes.KubeClientProvider
}

func NewStrongholdReadinessChecker(getter kubernetes.KubeClientProvider) *StrongholdReadinessChecker {
	return &StrongholdReadinessChecker{
		getter: getter,
	}
}

// IsReady checks that all replicas of the Stronghold StatefulSet are ready.
// The nodeName parameter is ignored because the check is cluster-wide.
// Returns true (skip) when the d8-stronghold namespace or the StatefulSet does not exist.
func (c *StrongholdReadinessChecker) IsReady(ctx context.Context, _ string) (bool, error) {
	kubeClient := c.getter.KubeClient()

	_, err := kubeClient.CoreV1().Namespaces().Get(ctx, strongholdNamespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.DebugLn("Namespace d8-stronghold not found, skipping Stronghold readiness check")
			return true, nil
		}
		return false, fmt.Errorf("failed to check d8-stronghold namespace: %w", err)
	}

	sts, err := kubeClient.AppsV1().StatefulSets(strongholdNamespace).Get(ctx, strongholdStatefulSet, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.DebugLn("StatefulSet stronghold not found in d8-stronghold namespace, skipping readiness check")
			return true, nil
		}
		return false, fmt.Errorf("failed to get Stronghold StatefulSet: %w", err)
	}

	desired := int32(1)
	if sts.Spec.Replicas != nil {
		desired = *sts.Spec.Replicas
	}
	ready := sts.Status.ReadyReplicas

	if ready < desired {
		log.InfoF("Stronghold StatefulSet: %d/%d replicas ready\n", ready, desired)
		return false, nil
	}

	log.InfoF("Stronghold StatefulSet is ready (%d/%d replicas)\n", ready, desired)
	return true, nil
}

func (c *StrongholdReadinessChecker) Name() string {
	return "Stronghold readiness"
}
