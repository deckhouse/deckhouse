// Copyright 2023 Flant JSC
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

package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

type LabelSelector struct {
	Label    string
	Operator selection.Operator
	Vals     []string
}

func GetLabelSelector(selectors []LabelSelector) (string, error) {
	if len(selectors) == 0 {
		return "", fmt.Errorf("Pass empty label selectors to GetLabelSelector")
	}

	requirements := make([]labels.Requirement, 0, len(selectors))

	for i, s := range selectors {
		r, err := labels.NewRequirement(s.Label, s.Operator, s.Vals)
		if err != nil {
			return "", fmt.Errorf(
				"Cannot create requirement for selector [%d] %s/%s[%s]: %w",
				i,
				s.Label,
				s.Operator,
				strings.Join(s.Vals, ", "),
				err,
			)
		}

		requirements = append(requirements, *r)
	}

	selector := labels.NewSelector()
	selector = selector.Add(requirements...)
	return selector.String(), nil
}

func GetMasterNodeGroupLabelSelector(selectors ...LabelSelector) (string, error) {
	withNg := []LabelSelector{
		{
			Label:    global.NodeGroupLabel,
			Operator: selection.Equals,
			Vals:     []string{global.MasterNodeGroupName},
		},
	}

	withNg = append(withNg, selectors...)

	return GetLabelSelector(withNg)
}

type KubeClientProvider interface {
	KubeClient() *client.KubernetesClient
}

// todo refactor it we need one provider with context
type KubeClientProviderWithCtx interface {
	KubeClientCtx(ctx context.Context) (*client.KubernetesClient, error)
}

var _ KubeClientProvider = &SimpleKubeClientGetter{}
var _ KubeClientProviderWithCtx = &SimpleKubeClientGetter{}

type SimpleKubeClientGetter struct {
	kubeCl *client.KubernetesClient
}

func NewSimpleKubeClientGetter(kubeCl *client.KubernetesClient) *SimpleKubeClientGetter {
	return &SimpleKubeClientGetter{kubeCl: kubeCl}
}

func (s *SimpleKubeClientGetter) KubeClient() *client.KubernetesClient {
	return s.kubeCl
}

func (s *SimpleKubeClientGetter) KubeClientCtx(context.Context) (*client.KubernetesClient, error) {
	return s.kubeCl, nil
}
