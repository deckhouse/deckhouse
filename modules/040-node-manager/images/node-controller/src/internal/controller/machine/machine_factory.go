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
	"fmt"

	capi "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type machineFactory struct{}

func NewMachineFactory() MachineFactory {
	return &machineFactory{}
}

func (f *machineFactory) NewMachine(obj client.Object) (Machine, error) {
	switch m := obj.(type) {
	case *mcmv1alpha1.Machine:
		return &mcmMachine{machine: m}, nil
	case *capi.Machine:
		return &capiMachine{machine: m}, nil
	default:
		return nil, fmt.Errorf("unsupported machine type: %T", obj)
	}
}

func (f *machineFactory) NewMachineFromRef(
	ctx context.Context,
	c client.Client,
	ref *deckhousev1alpha2.MachineRef,
) (Machine, error) {
	if ref == nil {
		return nil, fmt.Errorf("machine ref is nil")
	}
	if ref.Name == "" {
		return nil, fmt.Errorf("machine ref name is empty")
	}

	key := machineRefKey(ref)

	switch ref.APIVersion {
	case capi.GroupVersion.String():
		obj := &capi.Machine{}
		if err := c.Get(ctx, key, obj); err != nil {
			return nil, err
		}
		return f.NewMachine(obj)
	case mcmv1alpha1.SchemeGroupVersion.String():
		obj := &mcmv1alpha1.Machine{}
		if err := c.Get(ctx, key, obj); err != nil {
			return nil, err
		}
		return f.NewMachine(obj)
	default:
		return nil, fmt.Errorf("unsupported machine apiVersion: %q", ref.APIVersion)
	}
}

func newMachineRef(apiVersion, name string) *deckhousev1alpha2.MachineRef {
	return &deckhousev1alpha2.MachineRef{
		Kind:       "Machine",
		APIVersion: apiVersion,
		Name:       name,
		Namespace:  MachineNamespace,
	}
}

func machineRefKey(ref *deckhousev1alpha2.MachineRef) types.NamespacedName {
	namespace := ref.Namespace
	if namespace == "" {
		namespace = MachineNamespace
	}

	return types.NamespacedName{
		Namespace: namespace,
		Name:      ref.Name,
	}
}
