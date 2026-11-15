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

package hooks

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilnet "k8s.io/apimachinery/pkg/util/net"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

type reservedHostPortRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// defaultAPIServerNodePortRange is the range kube-apiserver uses without --service-node-port-range.
var defaultAPIServerNodePortRange = reservedHostPortRange{Min: 30000, Max: 32767}

type nodePortRangeSource struct {
	Range *reservedHostPortRange `json:"range,omitempty"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// Throttled in its own queue, so that waiting does not hold up the main queue.
	// Events that arrive while waiting are combined into one run.
	Queue: "/modules/deckhouse/reserved-host-port-ranges",
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 5 * time.Second,
		ExecutionBurst:       3,
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name: "node_port_apiserver_component", ApiVersion: "v1", Kind: "Pod",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}}},
			LabelSelector:     &metav1.LabelSelector{MatchLabels: map[string]string{"component": "kube-apiserver"}},
			FilterFunc:        filterAPIServerNodePortRange,
		},
		{
			Name: "node_port_cilium", ApiVersion: "v1", Kind: "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"d8-cni-cilium"}}},
			NameSelector:      &types.NameSelector{MatchNames: []string{"cilium-config"}},
			FilterFunc:        filterCiliumNodePortRange,
		},
	},
}, discoverReservedHostPortRanges)

func filterAPIServerNodePortRange(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod
	if err := sdk.FromUnstructured(obj, &pod); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}
	for _, container := range pod.Spec.Containers {
		if container.Name != "kube-apiserver" {
			continue
		}
		args := append(append([]string{}, container.Command...), container.Args...)
		value := ""
		for i, arg := range args {
			if strings.HasPrefix(arg, "--service-node-port-range=") {
				value = strings.TrimPrefix(arg, "--service-node-port-range=")
			} else if arg == "--service-node-port-range" && i+1 < len(args) {
				value = args[i+1]
			}
		}
		// kube-apiserver uses the default range without the flag, and accepts a blank value for it.
		if strings.TrimSpace(value) == "" {
			defaultRange := defaultAPIServerNodePortRange
			return nodePortRangeSource{Range: &defaultRange}, nil
		}
		var ports utilnet.PortRange
		if err := ports.Set(value); err != nil || ports.Base < 1 || ports.Size < 1 || ports.Base+ports.Size-1 > 65535 {
			return nil, fmt.Errorf("invalid NodePort range %q", value)
		}
		return nodePortRangeSource{Range: &reservedHostPortRange{Min: ports.Base, Max: ports.Base + ports.Size - 1}}, nil
	}
	// Not a kube-apiserver Pod after all.
	return nodePortRangeSource{}, nil
}

func filterCiliumNodePortRange(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm corev1.ConfigMap
	if err := sdk.FromUnstructured(obj, &cm); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}
	// Only native Cilium hostPort handling reserves this range. An absent
	// ConfigMap is not evidence that Cilium is installed or enabled.
	switch mode := cm.Data["kube-proxy-replacement"]; mode {
	case "true", "strict":
	// Without the key Cilium uses its default, false.
	case "", "false", "disabled":
		return nodePortRangeSource{}, nil
	default:
		return nil, fmt.Errorf("unknown kube-proxy-replacement %q", mode)
	}
	value, exists := cm.Data["node-port-range"]
	if !exists {
		value = "30000,32767"
	}
	return parseNodePortRangeSource(value, ",")
}

func parseNodePortRangeSource(value, separator string) (nodePortRangeSource, error) {
	parts := strings.Split(value, separator)
	if len(parts) == 2 {
		minPort, minErr := strconv.Atoi(strings.TrimSpace(parts[0]))
		maxPort, maxErr := strconv.Atoi(strings.TrimSpace(parts[1]))
		if minErr == nil && maxErr == nil && minPort >= 1 && maxPort <= 65535 && minPort <= maxPort {
			return nodePortRangeSource{Range: &reservedHostPortRange{Min: minPort, Max: maxPort}}, nil
		}
	}
	return nodePortRangeSource{}, fmt.Errorf("invalid NodePort range %q", value)
}

func discoverReservedHostPortRanges(_ context.Context, input *go_hook.HookInput) error {
	apiServerSources, err := sdkobjectpatch.UnmarshalToStruct[nodePortRangeSource](input.Snapshots, "node_port_apiserver_component")
	if err != nil {
		return fmt.Errorf("failed to unmarshal node_port_apiserver_component snapshot: %w", err)
	}
	ciliumSources, err := sdkobjectpatch.UnmarshalToStruct[nodePortRangeSource](input.Snapshots, "node_port_cilium")
	if err != nil {
		return fmt.Errorf("failed to unmarshal node_port_cilium snapshot: %w", err)
	}

	ranges := make([]reservedHostPortRange, 0)
	for _, source := range apiServerSources {
		if source.Range != nil {
			ranges = append(ranges, *source.Range)
		}
	}
	// Without a visible kube-apiserver Pod, e.g. with a managed control plane, assume the default range.
	if len(ranges) == 0 {
		input.Logger.Warn("kube-apiserver Pods not found, assuming the default NodePort range")
		ranges = append(ranges, defaultAPIServerNodePortRange)
	}
	for _, source := range ciliumSources {
		if source.Range != nil {
			ranges = append(ranges, *source.Range)
		}
	}

	sort.Slice(ranges, func(i, j int) bool { return ranges[i].Min < ranges[j].Min })
	merged := make([]reservedHostPortRange, 0, len(ranges))
	for _, current := range ranges {
		if len(merged) == 0 || current.Min > merged[len(merged)-1].Max+1 {
			merged = append(merged, current)
			continue
		}
		last := &merged[len(merged)-1]
		last.Max = max(last.Max, current.Max)
	}
	// Always replace the previous result, including when every source disappears.
	input.Values.Set("global.discovery.reservedHostPortRanges", merged)
	return nil
}
