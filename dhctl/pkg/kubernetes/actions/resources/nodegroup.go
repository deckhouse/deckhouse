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

package resources

import (
	"context"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources/readiness"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type NodeGroupsChecker struct {
	params constructorParams
}

func newNodeGroupsChecker(params constructorParams) (*NodeGroupsChecker, error) {
	return &NodeGroupsChecker{params: params}, nil
}

func (n *NodeGroupsChecker) Name() string {
	return "Waiting for NodeGroups become ready"
}

func (n *NodeGroupsChecker) Single() bool {
	return true
}

const (
	trueCondition  = "True"
	readyCondition = "Ready"
)

func (n *NodeGroupsChecker) IsReady(ctx context.Context) (bool, error) {
	readyCondition := readiness.Conditions{
		readyCondition: trueCondition,
	}
	waitAttempts := 3

	checker := readiness.NewByConditionsChecker(readyCondition, n.params.loggerProvider).
		WithWaitAttempts(waitAttempts).
		WithCheckAll(true)

	ready := true
	kubeCl, err := n.params.kubeProvider.KubeClientCtx(ctx)
	if err != nil {
		return false, err
	}
	ngs, err := entity.GetNodeGroups(ctx, kubeCl)
	if err != nil {
		return false, err
	}

	filtred := filterNodeGroups(ngs)
	for _, item := range filtred {
		r := false
		r, err = checker.IsReady(ctx, &item, item.GetName())
		if !r {
			ready = r
		}
	}

	return ready, err
}

func filterNodeGroups(ngs []unstructured.Unstructured) []unstructured.Unstructured {
	filtred := make([]unstructured.Unstructured, len(ngs)-1)
	for _, item := range ngs {
		// master NodeGroupd shouldn't be checked
		ng, err := unstructuredToNodeGroup(&item)
		if err != nil {
			continue
		}

		if ng.Name == "master" {
			continue
		}
		filtred = append(filtred, item)
	}

	return filtred
}
