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

package controller

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientpkg "sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"caps-controller-manager/internal/scope"
)

// NewMachineScope creates a new machine scope.
func NewMachineScope(ctx context.Context, client client.Client, config *rest.Config, staticMachine *infrav1.StaticMachine) (*scope.MachineScope, bool, error) {
	logger := ctrl.LoggerFrom(ctx)

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, client, staticMachine.ObjectMeta)
	if err != nil {
		return nil, false, err
	}
	if machine == nil {
		logger.Info("Machine Controller has not yet set OwnerRef")

		return nil, false, nil
	}

	nodeGroupLabel, ok := machine.Labels["node-group"]
	if !ok {
		patchHelper, err := patch.NewHelper(machine, client)
		if err != nil {
			return nil, false, errors.Wrap(err, "failed to init patch helper")
		}

		machine.Labels["node-group"] = staticMachine.Labels["node-group"]

		err = patchHelper.Patch(ctx, machine)
		if err != nil {
			return nil, false, errors.Wrap(err, "failed to patch Machine with node-group label")
		}
	} else {
		if nodeGroupLabel != staticMachine.Labels["node-group"] {
			return nil, false, errors.New("'node-group' label in StaticMachine and Machine are different")
		}
	}

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, client, machine.ObjectMeta)
	if err != nil {
		logger.Info("Machine is missing cluster label or cluster does not exist")

		return nil, false, nil
	}

	staticCluster := &infrav1.StaticCluster{}
	staticClusterNamespacedName := clientpkg.ObjectKey{
		Namespace: staticMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}

	// Fetch the StaticCluster.
	err = client.Get(ctx, staticClusterNamespacedName, staticCluster)
	if err != nil {
		logger.Info("StaticCluster is not available yet")

		return nil, false, nil
	}

	newScope, err := scope.NewScope(client, config, ctrl.LoggerFrom(ctx))
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to create new scope")
	}

	clusterScope, err := scope.NewClusterScope(newScope, cluster, staticCluster)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to create cluster scope")
	}

	newScope, err = scope.NewScope(client, config, ctrl.LoggerFrom(ctx))
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to create new scope")
	}

	machineScope, err := scope.NewMachineScope(newScope, clusterScope, machine, staticMachine)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to create machine scope")
	}

	return machineScope, true, nil
}
