/*
Copyright 2022 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	dsv1alpha1 "github.com/deckhouse/deckhouse/modules/400-descheduler/hooks/internal/v1alpha1"
)

const (
	deschedulerSpecsValuesPath = "descheduler.internal.deschedulers"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Queue:        "/modules/descheduler",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "deschedulers",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "Descheduler",
			FilterFunc: applyDeschedulerFilter,
		},
	},
}, getCRDsHandler)

type DeschedulerSnapshotItem struct {
	Name string
	Spec dsv1alpha1.DeschedulerSpec
}

func applyDeschedulerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ds := &dsv1alpha1.Descheduler{}

	err := sdk.FromUnstructured(obj, &ds)
	if err != nil {
		return nil, err
	}

	return DeschedulerSnapshotItem{
		Name: ds.Name,
		Spec: ds.Spec,
	}, nil
}

type InternalValuesDeschedulerSpec struct {
	Name           string                `json:"name" yaml:"name"`
	DefaultEvictor *DefaultEvictor       `json:"defaultEvictor,omitempty" yaml:"defaultEvictor,omitempty"`
	Strategies     dsv1alpha1.Strategies `json:"strategies" yaml:"strategies"`
}

type DefaultEvictor struct {
	NodeSelector      string                        `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	LabelSelector     *metav1.LabelSelector         `json:"labelSelector,omitempty" yaml:"labelSelector,omitempty"`
	PriorityThreshold *dsv1alpha1.PriorityThreshold `json:"priorityThreshold,omitempty" yaml:"priorityThreshold,omitempty"`
}

func getCRDsHandler(input *go_hook.HookInput) error {
	internalValues := make([]InternalValuesDeschedulerSpec, 0)
	for _, v := range input.Snapshots["deschedulers"] {
		item := v.(DeschedulerSnapshotItem)
		de := &DefaultEvictor{}
		if item.Spec.DefaultEvictor != nil {
			if item.Spec.DefaultEvictor.NodeSelector != nil {
				de.NodeSelector = metav1.FormatLabelSelector(item.Spec.DefaultEvictor.NodeSelector)
			}
			if item.Spec.DefaultEvictor.LabelSelector != nil {
				de.LabelSelector = item.Spec.DefaultEvictor.LabelSelector
			}
			if item.Spec.DefaultEvictor.PriorityThreshold != nil {
				de.PriorityThreshold = item.Spec.DefaultEvictor.PriorityThreshold
			}
		}

		internalValues = append(internalValues, InternalValuesDeschedulerSpec{
			Name:           item.Name,
			DefaultEvictor: de,
			Strategies:     item.Spec.Strategies,
		})
	}

	input.Values.Set(deschedulerSpecsValuesPath, internalValues)
	return nil
}
