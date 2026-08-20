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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/cloudprovider"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
)

type Reconciler struct {
	Client        client.Client
	Context       *Service
	DerivedStatus *derived_status.Service
}

func (r *Reconciler) Assemble(ctx context.Context) error {
	logger := log.FromContext(ctx)

	prior := r.readPriorNodeGroups(ctx)

	ngList := &v1.NodeGroupList{}
	if err := r.Client.List(ctx, ngList); err != nil {
		return fmt.Errorf("list nodegroups: %w", err)
	}

	// Loaded once for the whole context, not once per NodeGroup: resolving inside the loop meant
	// one registration read per NodeGroup on every write of the Secret.
	providers, err := cloudprovider.Load(ctx, r.Client)
	if err != nil {
		return err
	}

	nodeGroups := make([]map[string]interface{}, 0, len(ngList.Items))
	for i := range ngList.Items {
		ng := &ngList.Items[i]

		provider, err := providers.ForNodeGroup(ng)
		if err != nil {
			return fmt.Errorf("resolve the provider of NodeGroup %s: %w", ng.Name, err)
		}

		resolved, errStr, err := r.DerivedStatus.ResolveNodeGroup(ctx, ng, provider)
		if err != nil {
			return fmt.Errorf("resolve NodeGroup %s: %w", ng.Name, err)
		}

		if errStr != "" {
			logger.Info("NodeGroup failed validation", "nodeGroup", ng.Name, "error", errStr)
			if p, ok := prior[ng.Name]; ok {
				nodeGroups = append(nodeGroups, withProviderType(p, provider))
			}
			continue
		}

		nodeGroups = append(nodeGroups, resolved.ToMap())
	}

	sort.Slice(nodeGroups, func(i, j int) bool {
		return nodeGroupName(nodeGroups[i]) < nodeGroupName(nodeGroups[j])
	})

	setNodeGroupInfo(nodeGroups)

	return r.Context.WriteSecret(ctx, nodeGroups, providers)
}

// withProviderType refreshes the provider of an entry carried over from the last published context:
// an entry written before the key existed names none, and would render without cloud steps.
func withProviderType(entry map[string]interface{}, provider cloudprovider.Provider) map[string]interface{} {
	if provider.Type == "" {
		delete(entry, "cloudProviderType")
		return entry
	}

	entry["cloudProviderType"] = provider.Type

	return entry
}

// readPriorNodeGroups returns the entries of the currently published context, keyed by NodeGroup
// name. They stay raw parsed maps on purpose: the Secret may have been written by an older
// node-controller, and a shape this build does not model must survive the last-good fallback
// unchanged.
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
		nodeGroup, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if name := nodeGroupName(nodeGroup); name != "" {
			out[name] = nodeGroup
		}
	}
	return out
}

func nodeGroupName(nodeGroup map[string]interface{}) string {
	name, _ := nodeGroup["name"].(string)
	return name
}
