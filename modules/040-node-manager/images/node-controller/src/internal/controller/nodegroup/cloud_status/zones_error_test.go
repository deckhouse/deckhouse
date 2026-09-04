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

package cloud_status

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

// A zone count of 0 makes Min and Max 0, and a Min of 0 makes the NodeGroup Ready no matter how
// many nodes are actually up: conditionscalc compares readySchedulableNodes >= minPerAllZone. So an
// unreadable Secret must abort the pass rather than report zero zones and paint the group green.
//
// The interceptor is the last-resort tool the node-controller-testing skill allows here: a
// Forbidden response needs RBAC that envtest does not run.
func newDeniedZonesService(t *testing.T) *Service {
	t.Helper()

	c := fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if key.Name == common.CloudProviderSecretName {
					return apierrors.NewForbidden(
						schema.GroupResource{Resource: "secrets"}, key.Name, context.Canceled)
				}
				return cl.Get(ctx, key, obj, opts...)
			},
		}).
		Build()

	return &Service{Client: c}
}

func cloudEphemeralNodeGroup(minPerZone, maxPerZone int32) *v1.NodeGroup {
	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	ng.Spec.NodeType = v1.NodeTypeCloudEphemeral
	ng.Spec.CloudInstances = &v1.CloudInstancesSpec{MinPerZone: minPerZone, MaxPerZone: maxPerZone}
	return ng
}

func TestGetZonesCount_ForbiddenIsAnError(t *testing.T) {
	s := newDeniedZonesService(t)

	_, err := s.getZonesCount(t.Context(), cloudEphemeralNodeGroup(2, 4))

	require.ErrorContains(t, err, "read cloud provider secret")
}

func TestCompute_UnreadableSecretDoesNotReportZeroMin(t *testing.T) {
	s := newDeniedZonesService(t)

	_, err := s.Compute(t.Context(), cloudEphemeralNodeGroup(2, 4))

	require.Error(t, err)
}

func TestGetZonesCount_NotFoundIsZero(t *testing.T) {
	s := &Service{Client: newClient(t)}

	count, err := s.getZonesCount(t.Context(), cloudEphemeralNodeGroup(2, 4))

	require.NoError(t, err)
	require.Equal(t, int32(0), count)
}
