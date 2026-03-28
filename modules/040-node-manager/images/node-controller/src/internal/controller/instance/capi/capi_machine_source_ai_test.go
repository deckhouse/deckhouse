//go:build ai_tests

package capi

import (
	"context"
	"testing"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAI_CAPIEnsureInstanceFromMachine_Found(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, capiv1beta2.AddToScheme(scheme))
	require.NoError(t, deckhousev1alpha2.AddToScheme(scheme))

	capiMachine := &capiv1beta2.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-1",
			Namespace: "d8-cloud-provider-xxx",
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(capiMachine).Build()

	svc := NewCAPIMachineService()
	found, err := svc.EnsureInstanceFromMachine(context.Background(), c, types.NamespacedName{Namespace: "d8-cloud-provider-xxx", Name: "worker-1"})

	require.NoError(t, err)
	assert.True(t, found)
}

func TestAI_CAPIEnsureInstanceFromMachine_NotFound(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, capiv1beta2.AddToScheme(scheme))
	require.NoError(t, deckhousev1alpha2.AddToScheme(scheme))

	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	svc := NewCAPIMachineService()
	found, err := svc.EnsureInstanceFromMachine(context.Background(), c, types.NamespacedName{Namespace: "d8-cloud-provider-xxx", Name: "worker-1"})

	require.NoError(t, err)
	assert.False(t, found)
}
