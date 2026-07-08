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

package bashiblecontext

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
)

// nodeGroupListGVK is the list GVK used to enumerate every NodeGroup as an
// unstructured object, so the raw .spec (CRD-shaped, apiserver-pruned) can be
// passed to BuildElement verbatim — building the blob from the hand-rolled typed
// spec would diverge and break byte-parity.
var nodeGroupListGVK = schema.GroupVersionKind{Group: v1.GroupVersion.Group, Version: v1.GroupVersion.Version, Kind: "NodeGroupList"}

// Reconciler assembles the whole internal.nodeGroups blob from every NodeGroup
// and writes the bashible-apiserver-context Secret — the single-writer
// replacement for the get_crds hook + helm define bashible_input_data.
//
// ⚠ It must NOT be registered as an active controller while the helm define
// still renders the same Secret (dual-writer). The cutover — registering this
// and removing the helm define — must be atomic.
type Reconciler struct {
	Client        client.Client
	Context       *Service
	DerivedStatus *derived_status.Service
}

// Assemble lists every NodeGroup, builds its blob element (preserving the
// previously-stored element on validation failure, exactly like get_crds), and
// upserts the Secret. Elements are sorted by name for a deterministic payload.
func (r *Reconciler) Assemble(ctx context.Context) error {
	logger := log.FromContext(ctx)

	prior := r.readPriorNodeGroups(ctx)

	ngList := &unstructured.UnstructuredList{}
	ngList.SetGroupVersionKind(nodeGroupListGVK)
	if err := r.Client.List(ctx, ngList); err != nil {
		return fmt.Errorf("list nodegroups: %w", err)
	}

	elements := make([]map[string]interface{}, 0, len(ngList.Items))
	for i := range ngList.Items {
		obj := &ngList.Items[i]
		rawSpec, _ := obj.Object["spec"].(map[string]interface{})

		ng := &v1.NodeGroup{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, ng); err != nil {
			logger.Error(err, "skipping NodeGroup that failed to decode", "nodeGroup", obj.GetName())
			continue
		}

		element, errStr, err := r.DerivedStatus.BuildElement(ctx, ng, rawSpec)
		if err != nil {
			return fmt.Errorf("build blob element for NodeGroup %s: %w", ng.Name, err)
		}

		// Validation failure: reuse the previously-stored element to avoid
		// disruption, mirroring get_crds. With no prior element the NodeGroup is
		// omitted entirely (get_crds does the same `continue`).
		if errStr != "" {
			logger.Info("NodeGroup failed validation", "nodeGroup", ng.Name, "error", errStr)
			if p, ok := prior[ng.Name]; ok {
				elements = append(elements, p)
			}
			continue
		}

		elements = append(elements, element)
	}

	sort.Slice(elements, func(i, j int) bool {
		return blobName(elements[i]) < blobName(elements[j])
	})

	setNodeGroupInfo(elements)

	return r.Context.WriteSecret(ctx, elements)
}

// readPriorNodeGroups parses the current Secret's input.yaml and returns its
// nodeGroups keyed by name, so a NodeGroup that fails validation can keep its
// last-good element. An absent/unparseable Secret yields an empty map.
func (r *Reconciler) readPriorNodeGroups(ctx context.Context) map[string]map[string]interface{} {
	out := map[string]map[string]interface{}{}

	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: secretNamespace, Name: secretName}, secret); err != nil {
		return out
	}
	raw, ok := secret.Data[secretInputKey]
	if !ok {
		return out
	}
	var parsed map[string]interface{}
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return out
	}
	ngs, ok := parsed["nodeGroups"].([]interface{})
	if !ok {
		return out
	}
	for _, item := range ngs {
		element, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if name := blobName(element); name != "" {
			out[name] = element
		}
	}
	return out
}

func blobName(element map[string]interface{}) string {
	name, _ := element["name"].(string)
	return name
}
