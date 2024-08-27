/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package scopes

import (
	"context"
	"errors"
	"fmt"

	infrastructurev1alpha1 "github.com/deckhouse/deckhouse/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/patch"
)

// ClusterScope defines a scope defined around a cluster.
type ClusterScope struct {
	*Scope

	Cluster        *clusterv1.Cluster
	DynamixCluster *infrastructurev1alpha1.DynamixCluster
}

func NewClusterScope(
	base *Scope,
	cluster *clusterv1.Cluster,
	dynamixCluster *infrastructurev1alpha1.DynamixCluster,
) (*ClusterScope, error) {
	if base == nil {
		return nil, errors.New("Scope is required when creating a ClusterScope")
	}
	if cluster == nil {
		return nil, errors.New("Cluster is required when creating a ClusterScope")
	}
	if dynamixCluster == nil {
		return nil, errors.New("DynamixCluster is required when creating a ClusterScope")
	}

	patchHelper, err := patch.NewHelper(dynamixCluster, base.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to create patch helper: %w", err)
	}

	base.PatchHelper = patchHelper

	return &ClusterScope{
		Scope:          base,
		Cluster:        cluster,
		DynamixCluster: dynamixCluster,
	}, nil
}

// Fail marks the DynamixCluster as failed.
func (c *ClusterScope) Fail(reason capierrors.ClusterStatusError, err error) {
	c.DynamixCluster.Status.FailureReason = string(reason)
	c.DynamixCluster.Status.FailureMessage = err.Error()
}

// Patch updates the DynamixCluster resource.
func (c *ClusterScope) Patch(ctx context.Context) error {
	err := c.PatchHelper.Patch(ctx, c.DynamixCluster)
	if err != nil {
		return fmt.Errorf("failed to patch DynamixCluster: %w", err)
	}

	return nil
}
