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

// Package rules is the single implementation of the multi-tenancy scope of ClusterAuthorizationRules:
// what a rule says about the namespaces its subjects may act in, how the rules of a cluster fold into
// a directory keyed by subject, and what that directory answers for a request. The user-authz
// authorization webhook and permission-browser-apiserver both build their answers from this package,
// so the decision the API server enforces and the one the UI reports cannot drift apart.
package rules

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupVersionResource is the API resource the rules are read from.
var GroupVersionResource = schema.GroupVersionResource{
	Group:    "deckhouse.io",
	Version:  "v1",
	Resource: "clusterauthorizationrules",
}

// Subject is one subject of a rule. ServiceAccount subjects carry the namespace.
type Subject struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// NamespaceSelector is the spec.namespaceSelector of a rule: either a label selector over the
// namespaces or matchAny, which opens every namespace including the system ones.
type NamespaceSelector struct {
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
	MatchAny      bool                  `json:"matchAny,omitempty"`
}

// Applied reports whether the selector carries a label selector, which is the only form that
// restricts anything; matchAny is handled separately by the directory.
func (s *NamespaceSelector) Applied() bool {
	return s != nil && s.LabelSelector != nil
}

// Rule is the multi-tenancy view of a ClusterAuthorizationRule: the fields that decide which
// namespaces the subjects may act in. Everything else about the rule (access level, additional
// roles, port-forward) is RBAC and is the business of user-authz-controller.
type Rule struct {
	Name            string
	Generation      int64
	ResourceVersion string

	Subjects                      []Subject
	LimitNamespaces               []string
	NamespaceSelector             *NamespaceSelector
	AllowAccessToSystemNamespaces bool
}

// spec mirrors the part of the ClusterAuthorizationRule spec the rules need.
type spec struct {
	Subjects                      []Subject          `json:"subjects"`
	LimitNamespaces               []string           `json:"limitNamespaces"`
	NamespaceSelector             *NamespaceSelector `json:"namespaceSelector"`
	AllowAccessToSystemNamespaces bool               `json:"allowAccessToSystemNamespaces"`
}

// FromUnstructured reads a Rule out of a ClusterAuthorizationRule object.
func FromUnstructured(obj *unstructured.Unstructured) (Rule, error) {
	if obj == nil {
		return Rule{}, fmt.Errorf("nil object")
	}
	rule := Rule{
		Name:            obj.GetName(),
		Generation:      obj.GetGeneration(),
		ResourceVersion: obj.GetResourceVersion(),
	}
	rawSpec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return Rule{}, fmt.Errorf("rule %q: read spec: %w", rule.Name, err)
	}
	if !found {
		return rule, nil
	}
	var s spec
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(rawSpec, &s); err != nil {
		return Rule{}, fmt.Errorf("rule %q: decode spec: %w", rule.Name, err)
	}
	rule.Subjects = s.Subjects
	rule.LimitNamespaces = s.LimitNamespaces
	rule.NamespaceSelector = s.NamespaceSelector
	rule.AllowAccessToSystemNamespaces = s.AllowAccessToSystemNamespaces
	return rule, nil
}

// Project is an informer transform: it keeps of a ClusterAuthorizationRule only what FromUnstructured
// reads, so the cache does not hold managed fields, annotations, additional roles or the status of
// every rule of the cluster. Objects of other kinds and tombstones pass through untouched.
func Project(obj interface{}) (interface{}, error) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return obj, nil
	}
	projected := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": u.GetAPIVersion(),
		"kind":       u.GetKind(),
		"metadata": map[string]interface{}{
			"name":            u.GetName(),
			"resourceVersion": u.GetResourceVersion(),
			"generation":      u.GetGeneration(),
			"uid":             string(u.GetUID()),
		},
	}}
	if deletion := u.GetDeletionTimestamp(); deletion != nil {
		projected.SetDeletionTimestamp(deletion)
	}
	for _, field := range []string{"subjects", "limitNamespaces", "namespaceSelector", "allowAccessToSystemNamespaces"} {
		value, found, err := unstructured.NestedFieldNoCopy(u.Object, "spec", field)
		if err != nil || !found {
			continue
		}
		if err := unstructured.SetNestedField(projected.Object, runtime.DeepCopyJSONValue(value), "spec", field); err != nil {
			return nil, fmt.Errorf("rule %q: project spec.%s: %w", u.GetName(), field, err)
		}
	}
	return projected, nil
}

// SubjectKey is the directory key of a subject: the kind and the name the API server puts into
// user info. ServiceAccounts are keyed by their username system:serviceaccount:<namespace>:<name>.
func SubjectKey(s Subject) (kind, name string) {
	if s.Kind == "ServiceAccount" {
		return s.Kind, "system:serviceaccount:" + s.Namespace + ":" + s.Name
	}
	return s.Kind, s.Name
}
