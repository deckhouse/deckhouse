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

package machine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	capi "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
)

func TestNewMachineSupportedTypes(t *testing.T) {
	t.Parallel()

	f := NewMachineFactory()

	capiObj := &capi.Machine{}
	capiObj.Name = "capi"
	mCapi, err := f.NewMachine(capiObj)
	require.NoError(t, err)
	require.IsType(t, &capiMachine{}, mCapi)

	mcmObj := &mcmv1alpha1.Machine{}
	mcmObj.Name = "mcm"
	mMcm, err := f.NewMachine(mcmObj)
	require.NoError(t, err)
	require.IsType(t, &mcmMachine{}, mMcm)
}

func TestNewMachineFromRefMCMSuccess(t *testing.T) {
	t.Parallel()

	obj := &mcmv1alpha1.Machine{}
	obj.Name = "mcm-from-ref"
	obj.Namespace = MachineNamespace

	scheme := runtime.NewScheme()
	require.NoError(t, mcmv1alpha1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()

	m, err := NewMachineFactory().NewMachineFromRef(context.Background(), c, &deckhousev1alpha2.MachineRef{
		Name:       "mcm-from-ref",
		APIVersion: mcmv1alpha1.SchemeGroupVersion.String(),
		Namespace:  MachineNamespace,
	})
	require.NoError(t, err)
	require.Equal(t, "mcm-from-ref", m.GetName())
	require.IsType(t, &mcmMachine{}, m)
}

func TestNewMachineFromRefNotFoundPropagates(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, capi.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	_, err := NewMachineFactory().NewMachineFromRef(context.Background(), c, &deckhousev1alpha2.MachineRef{
		Name:       "absent",
		APIVersion: capi.GroupVersion.String(),
		Namespace:  MachineNamespace,
	})
	require.Error(t, err)
}
