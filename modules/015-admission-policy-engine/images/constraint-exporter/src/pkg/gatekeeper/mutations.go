/*
Copyright 2023 Flant JSC

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

package gatekeeper

import (
	"context"
	"log/slog"
	"maps"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	controllerClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	mutationsGroup               = "mutations.gatekeeper.sh"
	mutationsGroupVersionV1      = "v1"
	mutationsGroupVersionV1Alpha = "v1alpha1"
)

var mutationsGVs = []string{
	mutationsGroup + "/" + mutationsGroupVersionV1,
	mutationsGroup + "/" + mutationsGroupVersionV1Alpha,
}

type MutationMeta struct {
	Kind string
	Name string
}

// supportedMutationOperations mirrors the operations the Gatekeeper mutation webhook executes
// (supportedMutationOperations in gatekeeper/pkg/mutation/match/apply_to.go). An applyTo entry without
// operations, or with "*", stands for all of them.
var supportedMutationOperations = []string{"CREATE", "UPDATE"}

const operationAll = "*"

type MutationApplyTo struct {
	Operations []string `json:"operations,omitempty"`
}

type MutationSpec struct {
	Match   Match             `json:"match"`
	ApplyTo []MutationApplyTo `json:"applyTo,omitempty"`
}

type Mutation struct {
	Meta MutationMeta
	Spec MutationSpec
}

func (m Mutation) GetMatchKinds() []MatchKind {
	return m.Spec.Match.Kinds
}

// GetOperations returns the sorted admission operations that can trigger the mutation: the union of the
// operations of all its applyTo entries. Gatekeeper checks the operation per applyTo entry, so the union is a
// superset of what any single matched kind needs, which is safe for the webhook rules built from it.
// A mutation without applyTo (AssignMetadata) runs on every supported operation.
func (m Mutation) GetOperations() []string {
	if len(m.Spec.ApplyTo) == 0 {
		return slices.Clone(supportedMutationOperations)
	}

	ops := make(map[string]struct{}, len(supportedMutationOperations))
	for _, applyTo := range m.Spec.ApplyTo {
		if len(applyTo.Operations) == 0 || slices.Contains(applyTo.Operations, operationAll) {
			return slices.Clone(supportedMutationOperations)
		}

		for _, op := range applyTo.Operations {
			if slices.Contains(supportedMutationOperations, op) {
				ops[op] = struct{}{}
			}
		}
	}

	// The CRD accepts only supported operations, so an empty set means no entry could ever match.
	// Fall back to all of them rather than dropping the mutation from the webhook rules.
	if len(ops) == 0 {
		return slices.Clone(supportedMutationOperations)
	}

	return slices.Sorted(maps.Keys(ops))
}

type listClient interface {
	List(ctx context.Context, list controllerClient.ObjectList, opts ...controllerClient.ListOption) error
}

func GetMutations(cClient controllerClient.Client, client kubernetes.Interface) ([]Mutation, error) {
	return getMutations(cClient, client.Discovery(), mutationsGVs)
}

func getMutations(cClient listClient, discoveryClient discovery.DiscoveryInterface, groupVersions []string) ([]Mutation, error) {
	var (
		mutations []Mutation
		lastErr   error
	)

	seen := make(map[string]struct{})

	for _, mutationsGV := range groupVersions {
		c, err := discoveryClient.ServerResourcesForGroupVersion(mutationsGV)
		if err != nil {
			lastErr = err
			continue
		}

		gv, err := schema.ParseGroupVersion(c.GroupVersion)
		if err != nil {
			return nil, err
		}

		for _, r := range c.APIResources {
			if !canListResource(r) {
				continue
			}

			actual := &unstructured.UnstructuredList{}
			actual.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   mutationsGroup,
				Kind:    r.Kind,
				Version: gv.Version,
			})

			err = cClient.List(context.TODO(), actual)
			if err != nil {
				return nil, err
			}

			for _, item := range actual.Items {
				uniqKey := item.GetKind() + "/" + item.GetName()
				if _, ok := seen[uniqKey]; ok {
					continue
				}

				var mutation Mutation
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &mutation)
				if err != nil {
					slog.Error("decode mutation failed", "kind", item.GetKind(), "name", item.GetName(), "error", err)
					continue
				}

				mutation.Meta.Kind = item.GetKind()
				mutation.Meta.Name = item.GetName()
				mutations = append(mutations, mutation)
				seen[uniqKey] = struct{}{}
			}
		}
	}

	if len(mutations) == 0 && lastErr != nil {
		return nil, lastErr
	}

	return mutations, nil
}

func canListResource(resource metav1.APIResource) bool {
	for _, verb := range resource.Verbs {
		if verb == "list" {
			return true
		}
	}

	return false
}
