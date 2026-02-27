/*
Copyright 2025 Flant JSC

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

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	MachineStatusProgressing = "Progressing"
	MachineStatusReady       = "Ready"
	MachineStatusBlocked     = "Blocked"
)

const MachineNamespace = "d8-cloud-instance-manager"

type MachineFactory interface {
	NewMachine(obj client.Object) (Machine, error)
	NewMachineFromRef(ctx context.Context, c client.Client, ref *deckhousev1alpha2.MachineRef) (Machine, error)
}

type Machine interface {
	GetName() string
	GetNodeName() string
	GetNodeGroup() string
	GetMachineRef() *deckhousev1alpha2.MachineRef
	GetStatus() MachineStatus
	Exists(ctx context.Context, c client.Client) (bool, error)
	EnsureDeleted(ctx context.Context, c client.Client) (bool, error)
}

type MachineStatus struct {
	Phase         deckhousev1alpha2.InstancePhase
	MachineStatus string
	Conditions    []deckhousev1alpha2.InstanceCondition
}
