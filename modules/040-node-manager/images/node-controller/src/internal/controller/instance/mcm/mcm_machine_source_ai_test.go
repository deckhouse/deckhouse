//go:build ai_tests

package mcm

import (
	"context"
	"testing"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAI_MCMEnsureInstanceFromMachine_Found(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, mcmv1alpha1.AddToScheme(scheme))
	require.NoError(t, deckhousev1alpha2.AddToScheme(scheme))

	mcmMachine := &mcmv1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-1",
			Namespace: "d8-cloud-provider-xxx",
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mcmMachine).Build()

	svc := NewMCMMachineService()
	found, err := svc.EnsureInstanceFromMachine(context.Background(), c, types.NamespacedName{Namespace: "d8-cloud-provider-xxx", Name: "worker-1"})

	require.NoError(t, err)
	assert.True(t, found)
}

func TestAI_MCMEnsureInstanceFromMachine_NotFound(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, mcmv1alpha1.AddToScheme(scheme))
	require.NoError(t, deckhousev1alpha2.AddToScheme(scheme))

	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	svc := NewMCMMachineService()
	found, err := svc.EnsureInstanceFromMachine(context.Background(), c, types.NamespacedName{Namespace: "d8-cloud-provider-xxx", Name: "worker-1"})

	require.NoError(t, err)
	assert.False(t, found)
}
