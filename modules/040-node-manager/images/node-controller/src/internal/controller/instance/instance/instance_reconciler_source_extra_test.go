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

package instance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
)

func TestLinkedMachineStatusSkipsEmptyRef(t *testing.T) {
	t.Parallel()

	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).Build()
	svc := &InstanceService{client: c, machineFactory: machine.NewMachineFactory()}

	status, err := svc.linkedMachineStatus(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, sourceStatusSkipped, status)

	status, err = svc.linkedMachineStatus(context.Background(), &deckhousev1alpha2.MachineRef{Name: ""})
	require.NoError(t, err)
	require.Equal(t, sourceStatusSkipped, status)
}

func TestLinkedNodeStatusSkipsEmptyName(t *testing.T) {
	t.Parallel()

	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).Build()
	svc := &InstanceService{client: c, machineFactory: machine.NewMachineFactory()}

	status, err := svc.linkedNodeStatus(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, sourceStatusSkipped, status)
}
