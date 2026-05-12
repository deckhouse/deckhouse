/*
Copyright 2026 Flant JSC

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

package pool

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"caps-controller-manager/internal/event"
)

// StaticInstancePool defines a pool of static instances.
type StaticInstancePool struct {
	client.Client
	config   *rest.Config
	recorder *event.Recorder
}

// NewStaticInstancePool creates a new static instance pool.
func NewStaticInstancePool(client client.Client, config *rest.Config, recorder *event.Recorder) *StaticInstancePool {
	return &StaticInstancePool{
		Client:   client,
		config:   config,
		recorder: recorder,
	}
}

// PickStaticInstance picks a StaticInstance for the given StaticMachine.
func (p *StaticInstancePool) PickStaticInstance(ctx context.Context, staticMachine *infrav1.StaticMachine) (*deckhousev1.StaticInstance, error) {
	staticInstances, err := p.findStaticInstancesInPhase(
		ctx,
		staticMachine,
		deckhousev1.StaticInstanceStatusCurrentStatusPhasePending,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find static instances in pending phase: %w", err)
	}
	if len(staticInstances) == 0 {
		return nil, nil
	}

	staticInstance := staticInstances[rand.Intn(len(staticInstances))]
	return &staticInstance, nil
}

func (p *StaticInstancePool) findStaticInstancesInPhase(ctx context.Context, staticMachine *infrav1.StaticMachine, phase deckhousev1.StaticInstanceStatusCurrentStatusPhase) ([]deckhousev1.StaticInstance, error) {
	staticInstances := &deckhousev1.StaticInstanceList{}

	labelSelector, err := staticMachineLabelSelector(staticMachine)
	if err != nil {
		return nil, fmt.Errorf("failed to get label selector: %w", err)
	}

	err = p.List(
		ctx,
		staticInstances,
		client.MatchingLabelsSelector{Selector: labelSelector},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find static instances in phase '%s': %w", phase, err)
	}

	var staticInstancesInPhase []deckhousev1.StaticInstance

	for _, staticInstance := range staticInstances.Items {
		if staticInstance.Status.CurrentStatus == nil || staticInstance.Status.CurrentStatus.Phase != phase {
			continue
		}

		staticInstancesInPhase = append(staticInstancesInPhase, staticInstance)
	}

	return staticInstancesInPhase, nil
}

func staticMachineLabelSelector(staticMachine *infrav1.StaticMachine) (labels.Selector, error) {
	allowBootstrapRequirement, err := labels.NewRequirement("node.deckhouse.io/allow-bootstrap", selection.NotIn, []string{"false"})
	if err != nil {
		panic(err.Error())
	}

	if staticMachine.Spec.LabelSelector == nil {
		return labels.NewSelector().Add(*allowBootstrapRequirement), nil
	}

	labelSelector, err := metav1.LabelSelectorAsSelector(staticMachine.Spec.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("unable to convert StaticMachine label selector: %w", err)
	}

	requirements, _ := labelSelector.Requirements()

	for _, requirement := range requirements {
		if requirement.Key() == allowBootstrapRequirement.Key() {
			return nil, errors.New("label selector requirement for the 'node.deckhouse.io/allow-bootstrap' key can't be added manually")
		}
	}

	return labelSelector.Add(*allowBootstrapRequirement), nil
}
