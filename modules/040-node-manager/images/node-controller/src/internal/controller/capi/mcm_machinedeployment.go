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

package capi

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
)

type mcmMachineDeploymentInput struct {
	resolved         derived_status.ResolvedNodeGroup
	ngName           string
	zone             string
	mdName           string // {prefix-}{ng.name}-{hash}
	machineClassName string // {ng.name}-{hash}
	machineClassKind string // cloudProvider.machineClassKind
	region           string // cloudProvider.region (only used when nodeCapacity is set)
	checksum         string // checksum/machine-class annotation value
	replicas         int64
	awsSpot          bool // aws provider + instanceClass.spot → creationTimeout 5m
}

func buildMCMMachineDeployment(in mcmMachineDeploymentInput) *unstructured.Unstructured {
	// Everything below the resolved fields comes from the NodeGroup spec passthrough, which is
	// raw unstructured data all the way to the node.
	spec := in.resolved.Spec

	annotations := map[string]interface{}{
		"zone": in.zone,
	}
	if nodeCapacity, _ := in.resolved.NodeCapacity.(map[string]interface{}); nodeCapacity != nil {
		annotations["cluster-autoscaler.kubernetes.io/scale-from-zero"] = "true"
		annotations["cluster-autoscaler.kubernetes.io/node-region"] = in.region
		annotations["cluster-autoscaler.kubernetes.io/node-cpu"] = nestedString(nodeCapacity, "cpu")
		annotations["cluster-autoscaler.kubernetes.io/node-memory"] = nestedString(nodeCapacity, "memory")
		annotations["cluster-autoscaler.kubernetes.io/node-zone"] = in.zone
	}

	labels := map[string]interface{}{
		"heritage":   "deckhouse",
		"module":     "node-manager",
		"node-group": in.ngName,
	}

	instanceGroup := fmt.Sprintf("%s-%s", in.ngName, in.zone)

	cloudInstances := nestedMap(spec, "cloudInstances")
	maxSurge := intOrDefault(nestedInt32Ptr(cloudInstances, "maxSurgePerZone"), 1)
	maxUnavailable := intOrDefault(nestedInt32Ptr(cloudInstances, "maxUnavailablePerZone"), 0)

	drainTimeout, maxEvictRetries := mcmDrainTimeout(spec)

	nodeTemplate := nestedMap(spec, "nodeTemplate")

	nodeTemplateMeta := map[string]interface{}{
		"labels": mcmNodeTemplateLabels(in.ngName, nodeTemplate),
	}
	if ann := nestedMap(nodeTemplate, "annotations"); len(ann) > 0 {
		out := make(map[string]interface{}, len(ann))
		for k, v := range ann {
			out[k] = v
		}
		nodeTemplateMeta["annotations"] = out
	}
	nodeTemplateObj := map[string]interface{}{
		"metadata": nodeTemplateMeta,
	}
	if taints := mcmNodeTemplateTaints(nodeTemplate); taints != nil {
		nodeTemplateObj["spec"] = map[string]interface{}{"taints": taints}
	}

	templateSpec := map[string]interface{}{
		"class": map[string]interface{}{
			"kind": in.machineClassKind,
			"name": in.machineClassName,
		},
		"drainTimeout":    drainTimeout,
		"maxEvictRetries": maxEvictRetries,
		"nodeTemplate":    nodeTemplateObj,
	}
	if in.awsSpot {
		templateSpec["creationTimeout"] = "5m"
	}

	md := newUnstructured("machine.sapcloud.io", "v1alpha1", "MachineDeployment")
	md.Object["metadata"] = map[string]interface{}{
		"name":        in.mdName,
		"namespace":   capiNamespace,
		"labels":      labels,
		"annotations": annotations,
	}
	md.Object["spec"] = map[string]interface{}{
		"replicas":        in.replicas,
		"minReadySeconds": int64(300),
		"strategy": map[string]interface{}{
			"type": "RollingUpdate",
			"rollingUpdate": map[string]interface{}{
				"maxSurge":       int64(maxSurge),
				"maxUnavailable": int64(maxUnavailable),
			},
		},
		"selector": map[string]interface{}{
			"matchLabels": map[string]interface{}{
				"instance-group": instanceGroup,
			},
		},
		"template": map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"instance-group": instanceGroup,
				},
				"annotations": map[string]interface{}{
					"checksum/machine-class": in.checksum,
				},
			},
			"spec": templateSpec,
		},
	}
	return md
}

func mcmDrainTimeout(spec map[string]interface{}) (string, int64) {
	if cloudInstances := nestedMap(spec, "cloudInstances"); cloudInstances != nil {
		if q, ok := cloudInstances["quickShutdown"].(bool); ok && q {
			return "5m", 9
		}
	}
	// helm branched on `{{- else if $ng.nodeDrainTimeoutSecond }}`, so an explicit 0 was falsy
	// and fell through to the default. Honouring it literally would render drainTimeout: 0s
	// with maxEvictRetries: 0 — nodes deleted with no drain grace at all — and change
	// spec.template on upgrade, rolling every node of the group.
	if n, ok := nestedInt64(spec, "nodeDrainTimeoutSecond"); ok && n > 0 {
		return fmt.Sprintf("%ds", n), n / 20
	}
	return "600s", 30
}

func mcmNodeTemplateLabels(ngName string, nodeTemplate map[string]interface{}) map[string]interface{} {
	res := map[string]interface{}{
		"node-role.kubernetes.io/" + ngName: "",
		"node.deckhouse.io/group":           ngName,
		"node.deckhouse.io/type":            "CloudEphemeral",
	}
	for k, v := range nestedMap(nodeTemplate, "labels") {
		res[k] = v
	}
	return res
}

func mcmNodeTemplateTaints(nodeTemplate map[string]interface{}) []interface{} {
	raw, ok := nodeTemplate["taints"].([]interface{})
	if !ok || len(raw) == 0 {
		return nil
	}
	res := make([]interface{}, 0, len(raw))
	for _, item := range raw {
		t, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		taint := map[string]interface{}{
			"key":    nestedString(t, "key"),
			"effect": nestedString(t, "effect"),
		}
		if v := nestedString(t, "value"); v != "" {
			taint["value"] = v
		}
		res = append(res, taint)
	}
	return res
}

func nestedMap(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return nil
	}
	sub, _ := m[key].(map[string]interface{})
	return sub
}

// nestedString returns m[key] as a string, or "" when absent or not a string.
func nestedString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

func nestedInt64(m map[string]interface{}, key string) (int64, bool) {
	if m == nil {
		return 0, false
	}
	switch v := m[key].(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	default:
		return 0, false
	}
}

func nestedInt32Ptr(m map[string]interface{}, key string) *int32 {
	if n, ok := nestedInt64(m, key); ok {
		v := int32(n)
		return &v
	}
	return nil
}
