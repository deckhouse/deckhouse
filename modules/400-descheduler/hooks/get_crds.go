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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	dsv1alpha2 "github.com/deckhouse/deckhouse/modules/400-descheduler/hooks/internal/v1alpha2"
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
			ApiVersion: "deckhouse.io/v1alpha2",
			Kind:       "Descheduler",
			FilterFunc: applyDeschedulerFilter,
		},
	},
}, getCRDsHandler)

type DeschedulerSnapshotItem struct {
	Name string
	Spec dsv1alpha2.DeschedulerSpec
}

func applyDeschedulerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ds := &dsv1alpha2.Descheduler{}

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
	NodeLabelSelector      string                             `json:"nodeLabelSelector,omitempty" yaml:"nodeLabelSelector,omitempty"`
	PodLabelSelector       *metav1.LabelSelector              `json:"podLabelSelector,omitempty" yaml:"podLabelSelector,omitempty"`
	NamespaceLabelSelector *metav1.LabelSelector              `json:"namespaceLabelSelector,omitempty" yaml:"namespaceLabelSelector,omitempty"`
	PriorityClassThreshold *dsv1alpha2.PriorityClassThreshold `json:"priorityClassThreshold,omitempty" yaml:"priorityClassThreshold,omitempty"`
	EvictLocalStoragePods  *dsv1alpha2.EvictLocalStoragePods  `json:"evictLocalStoragePods,omitempty" yaml:"evictLocalStoragePods,omitempty"`
	Strategies             dsv1alpha2.Strategies              `json:"strategies" yaml:"strategies"`
}

func getCRDsHandler(_ context.Context, input *go_hook.HookInput) error {
	internalValues := make([]InternalValuesDeschedulerSpec, 0, len(input.Snapshots.Get("deschedulers")))
	for item, err := range sdkobjectpatch.SnapshotIter[DeschedulerSnapshotItem](input.Snapshots.Get("deschedulers")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'deschedulers' snapshots: %w", err)
		}

		ds := &InternalValuesDeschedulerSpec{
			Name:       item.Name,
			Strategies: item.Spec.Strategies,
		}
		if item.Spec.NodeSelector != "" {
			ds.NodeLabelSelector = item.Spec.NodeSelector
		} else if item.Spec.NodeLabelSelector != nil {
			ds.NodeLabelSelector = metav1.FormatLabelSelector(item.Spec.NodeLabelSelector)
		}

		if item.Spec.PodLabelSelector != nil {
			ds.PodLabelSelector = item.Spec.PodLabelSelector
		}
		if item.Spec.NamespaceLabelSelector != nil {
			ds.NamespaceLabelSelector = item.Spec.NamespaceLabelSelector
		}
		if item.Spec.PriorityClassThreshold != nil {
			ds.PriorityClassThreshold = item.Spec.PriorityClassThreshold
		}
		if item.Spec.EvictLocalStoragePods != nil {
			ds.EvictLocalStoragePods = item.Spec.EvictLocalStoragePods
		}

		internalValues = append(internalValues, *ds)
	}

	input.Values.Set(deschedulerSpecsValuesPath, internalValues)
	return nil
}
