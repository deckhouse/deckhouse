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

package scope

import (
	"context"

	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/patch"

	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
)

// ClusterScope defines a scope defined around a cluster.
type ClusterScope struct {
	*Scope

	Cluster       *clusterv1.Cluster
	StaticCluster *infrav1.StaticCluster
}

// NewClusterScope creates a new cluster.
func NewClusterScope(
	scope *Scope,
	cluster *clusterv1.Cluster,
	staticCluster *infrav1.StaticCluster,
) (*ClusterScope, error) {
	if scope == nil {
		return nil, errors.New("Scope is required when creating a ClusterScope")
	}
	if cluster == nil {
		return nil, errors.New("Cluster is required when creating a ClusterScope")
	}
	if staticCluster == nil {
		return nil, errors.New("StaticCluster is required when creating a ClusterScope")
	}

	patchHelper, err := patch.NewHelper(staticCluster, scope.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	scope.PatchHelper = patchHelper

	return &ClusterScope{
		Scope:         scope,
		Cluster:       cluster,
		StaticCluster: staticCluster,
	}, nil
}

// Fail marks the StaticCluster as failed.
func (c *ClusterScope) Fail(reason capierrors.ClusterStatusError, err error) {
	c.StaticCluster.Status.FailureReason = string(reason)
	c.StaticCluster.Status.FailureMessage = err.Error()
}

// Patch updates the StaticCluster resource.
func (c *ClusterScope) Patch(ctx context.Context) error {
	err := c.PatchHelper.Patch(ctx, c.StaticCluster)
	if err != nil {
		return errors.Wrap(err, "failed to patch StaticCluster")
	}

	return nil
}
