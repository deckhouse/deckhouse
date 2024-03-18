/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package scopes

import (
	"context"
	"errors"
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/patch"

	infrastructurev1alpha1 "github.com/deckhouse/deckhouse/api/v1alpha1"
)

// ClusterScope defines a scope defined around a cluster.
type ClusterScope struct {
	*Scope

	Cluster      *clusterv1.Cluster
	ZvirtCluster *infrastructurev1alpha1.ZvirtCluster
}

func NewClusterScope(
	base *Scope,
	cluster *clusterv1.Cluster,
	zvirtCluster *infrastructurev1alpha1.ZvirtCluster,
) (*ClusterScope, error) {
	if base == nil {
		return nil, errors.New("Scope is required when creating a ClusterScope")
	}
	if cluster == nil {
		return nil, errors.New("Cluster is required when creating a ClusterScope")
	}
	if zvirtCluster == nil {
		return nil, errors.New("ZvirtCluster is required when creating a ClusterScope")
	}

	patchHelper, err := patch.NewHelper(zvirtCluster, base.Client)
	if err != nil {
		return nil, fmt.Errorf("Failed to create patch helper: %w", err)
	}

	base.PatchHelper = patchHelper

	return &ClusterScope{
		Scope:        base,
		Cluster:      cluster,
		ZvirtCluster: zvirtCluster,
	}, nil
}

// Fail marks the StaticCluster as failed.
func (c *ClusterScope) Fail(reason capierrors.ClusterStatusError, err error) {
	c.ZvirtCluster.Status.FailureReason = string(reason)
	c.ZvirtCluster.Status.FailureMessage = err.Error()
}

// Patch updates the StaticCluster resource.
func (c *ClusterScope) Patch(ctx context.Context) error {
	err := c.PatchHelper.Patch(ctx, c.ZvirtCluster)
	if err != nil {
		return fmt.Errorf("Failed to patch ZvirtCluster: %w", err)
	}

	return nil
}
