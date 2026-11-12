// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package objectpatcher is the patcher of the hooks' patches. Under Module v2 it intercepts the
// hooks' patches of the Module status (moduleStatusClient).
package objectpatcher

import (
	"context"
	"encoding/json"
	"strings"

	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// New creates the patcher of the hooks' patches over client.
func New(client objectpatch.KubeClient, moduleV2 bool, logger *log.Logger) *objectpatch.ObjectPatcher {
	if moduleV2 {
		client = moduleStatusClient{KubeClient: client}
	}

	return objectpatch.NewObjectPatcher(client, logger)
}

// moduleStatusClient is the hooks' patcher client under Module v2. The readiness hook of a
// module-sdk older than the probe protocol reports through the IsReady condition of its Module,
// patched in v1alpha1. Under Module v2 the Module is stored in v1beta1 and its status belongs to the
// status service; the conversion of the patched object back to v1beta1 drops the status, so the
// patch would erase it. The patch never reaches the API: IsReady=False becomes the error of the
// hook's run (hooks.ProbeFailedError), the run's outcome is the probe's verdict, and anything else
// in it is dropped.
type moduleStatusClient struct {
	objectpatch.KubeClient
}

func (c moduleStatusClient) Dynamic() dynamic.Interface {
	return moduleStatusDynamic{Interface: c.KubeClient.Dynamic()}
}

type moduleStatusDynamic struct {
	dynamic.Interface
}

func (d moduleStatusDynamic) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	ri := d.Interface.Resource(gvr)
	if gvr != v1alpha1.ModuleGVR {
		return ri
	}

	return moduleStatusResources{NamespaceableResourceInterface: ri}
}

// moduleStatusResources is the Modules of v1alpha1; the patcher addresses every object through
// Namespace, "" for a cluster-scoped one.
type moduleStatusResources struct {
	dynamic.NamespaceableResourceInterface
}

func (r moduleStatusResources) Namespace(ns string) dynamic.ResourceInterface {
	return moduleStatusResource{ResourceInterface: r.NamespaceableResourceInterface.Namespace(ns)}
}

type moduleStatusResource struct {
	dynamic.ResourceInterface
}

func (r moduleStatusResource) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	// module-sdk names the subresource "/status".
	if len(subresources) != 1 || strings.Trim(subresources[0], "/") != "status" {
		return r.ResourceInterface.Patch(ctx, name, pt, data, opts, subresources...)
	}

	return nil, readinessVerdict(data)
}

// readinessVerdict is the verdict a status patch carries: an error when its IsReady condition is
// False, nil when it is True or the patch has none.
func readinessVerdict(patch []byte) error {
	var status struct {
		Status struct {
			Conditions []struct {
				Type    string `json:"type"`
				Status  string `json:"status"`
				Message string `json:"message"`
			} `json:"conditions"`
		} `json:"status"`
	}

	// Not a merge patch of the status: nothing to read, and it is dropped all the same.
	if json.Unmarshal(patch, &status) != nil {
		return nil
	}

	for _, cond := range status.Status.Conditions {
		if cond.Type == v1alpha1.ModuleConditionIsReady && cond.Status == string(metav1.ConditionFalse) {
			return &hooks.ProbeFailedError{Message: cond.Message}
		}
	}

	return nil
}
