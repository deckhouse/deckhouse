/*
Copyright 2023 Flant JSC

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
	"math/rand"

	"github.com/pkg/errors"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	"caps-controller-manager/internal/event"
	"caps-controller-manager/internal/scope"
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
func (p *StaticInstancePool) PickStaticInstance(
	ctx context.Context,
	machineScope *scope.MachineScope,
) (*scope.InstanceScope, bool, error) {
	staticInstances, err := p.findStaticInstancesInPhase(
		ctx,
		machineScope,
		deckhousev1.StaticInstanceStatusCurrentStatusPhasePending,
	)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to find static instances in pending phase")
	}
	if len(staticInstances) == 0 {
		return nil, false, nil
	}

	staticInstance := staticInstances[rand.Intn(len(staticInstances))]

	newScope, err := scope.NewScope(p.Client, p.config, ctrl.LoggerFrom(ctx))
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to create scope")
	}

	instanceScope, err := scope.NewInstanceScope(newScope, &staticInstance, ctx)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to create instance scope")
	}

	instanceScope.MachineScope = machineScope

	err = instanceScope.LoadSSHCredentials(ctx, p.recorder)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to load SSHCredentials")
	}

	return instanceScope, true, nil
}

func (p *StaticInstancePool) findStaticInstancesInPhase(
	ctx context.Context,
	machineScope *scope.MachineScope,
	phase deckhousev1.StaticInstanceStatusCurrentStatusPhase,
) ([]deckhousev1.StaticInstance, error) {
	staticInstances := &deckhousev1.StaticInstanceList{}

	labelSelector, err := machineScope.LabelSelector()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get label selector")
	}

	err = p.List(
		ctx,
		staticInstances,
		client.MatchingLabelsSelector{Selector: labelSelector},
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find static instances in phase '%s'", phase)
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
