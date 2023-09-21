package controller

import (
	infrav1 "cloud-provider-static/api/infrastructure/v1alpha1"
	"cloud-provider-static/internal/scope"
	"context"
	"github.com/pkg/errors"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientpkg "sigs.k8s.io/controller-runtime/pkg/client"
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
