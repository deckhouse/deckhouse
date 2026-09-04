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

package project

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/namespaces"
)

const upmeterNamespacePrefix = "upmeter-"

// VirtualProjectName is the virtual project that inventories this namespace, or
// empty if the namespace already belongs to a real project.
func VirtualProjectName(obj metav1.Object) string {
	if _, owned := obj.GetLabels()[v1alpha3.ResourceLabelProject]; owned {
		return ""
	}
	if IsDeckhouseInventory(obj) {
		return DeckhouseProjectName
	}
	return DefaultProjectName
}

// IsDeckhouseInventory reports whether an unowned namespace is a platform/system
// one and must be counted on the virtual deckhouse project — not adopted, and
// not dumped into virtual default.
func IsDeckhouseInventory(obj metav1.Object) bool {
	name := obj.GetName()
	if strings.HasPrefix(name, DeckhouseNamespacePrefix) ||
		strings.HasPrefix(name, KubernetesNamespacePrefix) ||
		strings.HasPrefix(name, upmeterNamespacePrefix) ||
		namespaces.IsSystem(name) {
		return true
	}
	switch obj.GetLabels()[v1alpha3.ResourceLabelHeritage] {
	case v1alpha3.ResourceHeritageDeckhouse, v1alpha3.ResourceHeritageUpmeter:
		return true
	}
	return false
}

// realProjectNames is every non-virtual Project. Adopt creates that object
// before Helm can label the namespace; HandleVirtual must not keep those
// namespaces on virtual default.
func (m *Manager) realProjectNames(ctx context.Context) (map[string]struct{}, error) {
	list := new(v1alpha3.ProjectList)
	if err := m.client.List(ctx, list); err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	out := make(map[string]struct{}, len(list.Items))
	for i := range list.Items {
		p := &list.Items[i]
		if isVirtualInventory(p) {
			continue
		}
		out[p.Name] = struct{}{}
	}
	return out, nil
}

func isVirtualInventory(p *v1alpha3.Project) bool {
	if p.Labels[v1alpha3.ProjectLabelVirtualProject] == "true" {
		return true
	}
	return p.Name == DeckhouseProjectName || p.Name == DefaultProjectName
}
