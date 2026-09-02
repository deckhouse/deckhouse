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

package derived_status

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// The cloud-provider Secret decides whether a NodeGroup has a cloud at all. A read failure used to
// be indistinguishable from "no cloud provider": the empty map made instanceClassKind empty, the
// checks returned Processed=false with an empty Error, and the bashible context — whose last-good
// fallback keys off a non-empty Error — published the NodeGroup without instanceClass. That is a
// configuration-checksum shift on every node of the cluster.
//
// These tests assert the resulting values, not which calls were made. The fake client with an
// interceptor is the last-resort tool the node-controller-testing skill allows for a branch that
// cannot be reached in envtest: producing a Forbidden needs RBAC that envtest does not run.
func newDeniedSecretService(t *testing.T, deniedNames ...string) *Service {
	t.Helper()

	denied := make(map[string]struct{}, len(deniedNames))
	for _, name := range deniedNames {
		denied[name] = struct{}{}
	}

	c := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if _, ok := denied[key.Name]; ok {
					return apierrors.NewForbidden(
						schema.GroupResource{Resource: "secrets"}, key.Name, context.Canceled)
				}
				return cl.Get(ctx, key, obj, opts...)
			},
		}).
		Build()

	return &Service{Client: c}
}

func TestReadCloudProviderData_ForbiddenIsAnError(t *testing.T) {
	s := newDeniedSecretService(t, cloudProviderSecretName)

	_, err := s.readCloudProviderData(t.Context())

	require.ErrorContains(t, err, "read cloud provider secret")
}

func TestReadCloudProviderData_NotFoundIsEmpty(t *testing.T) {
	s := newTestService(t)

	data, err := s.readCloudProviderData(t.Context())

	require.NoError(t, err)
	require.Empty(t, data)
}

func TestComputeWithCloudChecks_UnreadableSecretAbortsInsteadOfPublishing(t *testing.T) {
	s := newDeniedSecretService(t, cloudProviderSecretName)
	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	ng.Spec.NodeType = v1.NodeTypeCloudEphemeral

	_, _, err := s.ComputeWithCloudChecks(t.Context(), ng)

	require.Error(t, err)
}

func TestReadClusterConfiguration_ForbiddenIsAnError(t *testing.T) {
	s := newDeniedSecretService(t, clusterConfigSecretName)

	_, _, err := s.readClusterConfiguration(t.Context())

	require.ErrorContains(t, err, "read cluster configuration secret")
}

// Raw, not newTestService: that helper seeds the cluster ConfigMap, which is where the version comes
// from now, and this case is about both sources being absent.
func TestReadClusterConfiguration_NotFoundIsEmpty(t *testing.T) {
	s := newTestServiceRaw(t)

	version, cri, err := s.readClusterConfiguration(t.Context())

	require.NoError(t, err)
	require.Nil(t, version)
	require.Empty(t, cri)
}

// The version has its own source now, so it needs its own unavailability case: an unreadable
// ConfigMap must abort the pass rather than publish a NodeGroup with no version clamp.
func TestReadTargetKubernetesVersion_ForbiddenIsAnError(t *testing.T) {
	s := newDeniedSecretService(t, clusterKubernetesConfigMapName)

	_, err := s.readTargetKubernetesVersion(t.Context())

	require.ErrorContains(t, err, "read cluster kubernetes configmap")
}

func TestReadClusterUUID_ForbiddenIsAnError(t *testing.T) {
	s := newDeniedSecretService(t, clusterUUIDConfigMapName)

	_, err := s.readClusterUUID(t.Context())

	require.ErrorContains(t, err, "read cluster uuid configmap")
}

func TestReadClusterUUID_NotFoundIsEmpty(t *testing.T) {
	s := newTestService(t)

	uuid, err := s.readClusterUUID(t.Context())

	require.NoError(t, err)
	require.Empty(t, uuid)
}

// A cloud provider Secret that is merely absent is a static cluster, not a failure.
func TestComputeWithCloudChecks_AbsentSecretIsNotAnError(t *testing.T) {
	s := newTestService(t)
	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	ng.Spec.NodeType = v1.NodeTypeCloudEphemeral

	_, check, err := s.ComputeWithCloudChecks(t.Context(), ng)

	require.NoError(t, err)
	require.False(t, check.Processed)
	require.Empty(t, check.Error)
}

// A cluster can legitimately lack a whole kind: one without MCM has no MachineDeployment CRD, and a
// provider that ships no instance-type catalog has no InstanceTypesCatalog CRD. Both must read as
// "nothing there" — treating an uninstalled CRD as unavailable aborts the reconcile, and for a
// scale-from-zero NodeGroup that stops its MachineDeployment from ever being rendered.
//
// The behavioural guard for this is the CAPI envtest suite (it installs neither CRD and did fail
// exactly this way); here we pin the classification itself, which is where the mistake was made.
func TestIsAbsent(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expAbsent bool
	}{
		{
			name:      "object missing",
			err:       apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, "x"),
			expAbsent: true,
		},
		{
			name:      "CRD never installed",
			err:       &meta.NoKindMatchError{GroupKind: schema.GroupKind{Group: "machine.sapcloud.io", Kind: "MachineDeployment"}},
			expAbsent: true,
		},
		{
			name:      "kind absent from the scheme",
			err:       runtime.NewNotRegisteredErrForKind("test", schema.GroupVersionKind{Kind: "InstanceTypesCatalog"}),
			expAbsent: true,
		},
		{
			name:      "forbidden is unavailable, not absent",
			err:       apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "x", context.Canceled),
			expAbsent: false,
		},
		{
			name:      "timeout is unavailable, not absent",
			err:       apierrors.NewTimeoutError("too slow", 1),
			expAbsent: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expAbsent, isAbsent(tc.err))
		})
	}
}
