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
	Name                   string                             `json:"name" yaml:"name"`
	nodeLabelSelector      string                             `json:"nodeLabelSelector,omitempty" yaml:"nodeLabelSelector,omitempty"`
	PodLabelSelector       *metav1.LabelSelector              `json:"podLabelSelector,omitempty" yaml:"podLabelSelector,omitempty"`
	namespaceLabelSelector *metav1.LabelSelector              `json:"namespaceLabelSelector,omitempty" yaml:"namespaceLabelSelector,omitempty"`
	PriorityClassThreshold *dsv1alpha1.PriorityClassThreshold `json:"priorityClassThreshold,omitempty" yaml:"priorityClassThreshold,omitempty"`
	Strategies             dsv1alpha1.Strategies              `json:"strategies" yaml:"strategies"`
}

func getCRDsHandler(input *go_hook.HookInput) error {
	internalValues := make([]InternalValuesDeschedulerSpec, 0, len(input.Snapshots["deschedulers"]))
	for _, v := range input.Snapshots["deschedulers"] {
		item := v.(DeschedulerSnapshotItem)
		ds := &InternalValuesDeschedulerSpec{
			Name:       item.Name,
			Strategies: item.Spec.Strategies,
		}
		if item.Spec.nodeLabelSelector != nil {
			ds.nodeLabelSelector = metav1.FormatLabelSelector(item.Spec.nodeLabelSelector)
		}
		if item.Spec.PodLabelSelector != nil {
			ds.PodLabelSelector = item.Spec.PodLabelSelector
		}
		if item.Spec.namespaceLabelSelector != nil {
			ds.namespaceLabelSelector = item.Spec.namespaceLabelSelector
		}
		if item.Spec.PriorityClassThreshold != nil {
			ds.PriorityClassThreshold = item.Spec.PriorityClassThreshold
		}

		internalValues = append(internalValues, *ds)
	}

	input.Values.Set(deschedulerSpecsValuesPath, internalValues)
	return nil
}
