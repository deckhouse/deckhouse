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

// Nodes carrying container runtime registry configuration this module did not write.
//
// The warning exists because of a delay, and the delay is the whole problem. On containerd v1 an
// operator's registry configuration lives in `/etc/containerd/conf.d/*.toml`, which bashible merges into
// `config.toml`. While this module manages the registry, step 032 refuses such a file outright —
// "configure them in /etc/containerd/registry.d instead" — because two writers of a node's registry
// configuration is the confusion this implementation exists to remove.
//
// What the refusal does NOT do is happen when the file appears. Measured on a v1 stand: the file was
// planted, bashible said "Configuration is in sync, nothing to do", `config.toml` was left as it was, and
// the configuration simply had no effect. Then the node's configuration checksum was removed — which is
// what the periodic full run does by itself, within about four hours — and step 032 refused within
// fifteen seconds, failing in a retry loop from then on.
//
// So an operator who writes that file sees nothing happen, and hours later, in a moment unconnected to
// anything they did, a node stops converging. An immediate refusal would be kinder. Since the refusal
// cannot be made immediate from here — it belongs to a bashible step that only runs when it runs — the
// next best thing is to say so out loud the moment the node reports carrying such a file.
//
// The signal is already there: step 091 labels every node with
// `node.deckhouse.io/containerd-config-registry`, `custom` when a conf.d file carries registry fields and
// `default` when none does. This hook turns that label into a metric while this module manages the
// registry, and into nothing when it does not — on a cluster the module manages nothing on, the same file
// is merged normally and there is nothing to warn about.
package v2

import (
	"context"
	"fmt"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	// ForeignConfigMetric names the nodes, one series each: an operator fixes this per node, and a
	// count would not tell them which.
	ForeignConfigMetric      = "d8_registry_node_foreign_registry_config"
	foreignConfigMetricGroup = "d8_registry_node_foreign_registry_config"

	// ForeignConfigLabel is set by bashible step 091 on every node.
	ForeignConfigLabel = "node.deckhouse.io/containerd-config-registry"

	// ForeignConfigLabelValue is what that label says when a conf.d file carries registry fields.
	ForeignConfigLabelValue = "custom"

	foreignConfigSnapName = "nodes-with-foreign-registry-config"
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 8},
		Queue:        "/modules/registry/v2",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				// Selected by the label rather than filtered afterwards: on a large cluster this is
				// the difference between watching every node and watching the few that matter.
				Name:       foreignConfigSnapName,
				ApiVersion: "v1",
				Kind:       "Node",
				LabelSelector: &v1meta.LabelSelector{
					MatchLabels: map[string]string{ForeignConfigLabel: ForeignConfigLabelValue},
				},
				FilterFunc: filterForeignConfigNode,
			},
		},
	},
	handleForeignConfig,
)

func filterForeignConfigNode(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// Only the name. What the file contains is on the node, and reading it from here would be a second
	// opinion about something the node already answered.
	return obj.GetName(), nil
}

func handleForeignConfig(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(foreignConfigMetricGroup)

	if !IsActive(input) {
		// The module manages nothing here, so nobody refuses that file and it works as it always did.
		return nil
	}

	nodes, err := helpers.SnapshotToList[string](input, foreignConfigSnapName)
	if err != nil {
		return fmt.Errorf("reading the nodes: %w", err)
	}
	sort.Strings(nodes)

	for _, node := range nodes {
		input.MetricsCollector.Set(ForeignConfigMetric, 1,
			map[string]string{"node": node},
			metrics.WithGroup(foreignConfigMetricGroup))
	}

	if len(nodes) > 0 {
		input.Logger.Warn("nodes carry container runtime registry configuration this module did not write; "+
			"their next full bashible run will refuse it and stop converging",
			"nodes", nodes)
	}

	return nil
}
