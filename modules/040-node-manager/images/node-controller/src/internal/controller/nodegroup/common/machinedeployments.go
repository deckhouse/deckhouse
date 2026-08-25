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

package common

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// MachineDeployments reports which engines already have MachineDeployments for a NodeGroup. It is
// what the group actually runs on, as opposed to what its spec and the provider default to.
type MachineDeployments struct {
	MCM  bool
	CAPI bool
}

// ListMachineDeployments returns the MachineDeployments of one NodeGroup, of one engine. A missing
// CRD yields an empty list, not an error: CSE ships without machine-controller-manager.
func ListMachineDeployments(
	ctx context.Context, reader client.Reader, gvk schema.GroupVersionKind, ngName string,
) (*unstructured.UnstructuredList, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))

	err := reader.List(ctx, list,
		client.InNamespace(nodecommon.MachineNamespace),
		client.MatchingLabels{MachineDeploymentNodeGroupLabel: ngName},
	)
	if err == nil {
		return list, nil
	}
	if meta.IsNoMatchError(err) {
		return &unstructured.UnstructuredList{}, nil
	}
	return nil, fmt.Errorf("list %s of NodeGroup %s: %w", gvk.Kind, ngName, err)
}

// FindMachineDeployments answers which engine a NodeGroup already runs on.
func FindMachineDeployments(ctx context.Context, reader client.Reader, ngName string) (MachineDeployments, error) {
	mcm, err := ListMachineDeployments(ctx, reader, MCMMachineDeploymentGVK, ngName)
	if err != nil {
		return MachineDeployments{}, err
	}
	capi, err := ListMachineDeployments(ctx, reader, CAPIMachineDeploymentGVK, ngName)
	if err != nil {
		return MachineDeployments{}, err
	}
	return MachineDeployments{MCM: len(mcm.Items) > 0, CAPI: len(capi.Items) > 0}, nil
}
