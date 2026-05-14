/*
Copyright 2025 Flant JSC

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

package config

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
)

const controlPlaneManagerModuleName = "control-plane-manager"

// controlPlaneManagerSettings returns the control-plane-manager ModuleConfig settings
// ready for template rendering. Returns nil when the ModuleConfig is absent.
// resourcesRequests.{cpu,memory} are replaced with milliCPU and memoryBytes (int64)
// so templates can use them directly in arithmetic.
func (m *MetaConfig) controlPlaneManagerSettings() (map[string]interface{}, error) {
	var mc *ModuleConfig
	for _, c := range m.ModuleConfigs {
		if c.GetName() == controlPlaneManagerModuleName {
			mc = c
			break
		}
	}
	if mc == nil {
		return nil, nil
	}

	out := make(map[string]interface{}, len(mc.Spec.Settings))
	for k, v := range mc.Spec.Settings {
		out[k] = v
	}

	if rr, ok := mc.Spec.Settings["resourcesRequests"].(map[string]interface{}); ok {
		milliCPU, memoryBytes, err := parseResourceRequests(rr)
		if err != nil {
			return nil, fmt.Errorf("parse resourcesRequests: %w", err)
		}
		parsed := map[string]interface{}{}
		if milliCPU != 0 {
			parsed["milliCPU"] = milliCPU
		}
		if memoryBytes != 0 {
			parsed["memoryBytes"] = memoryBytes
		}
		out["resourcesRequests"] = parsed
	}

	return out, nil
}

func parseResourceRequests(rr map[string]interface{}) (milliCPU, memoryBytes int64, err error) {
	if cpu, _ := rr["cpu"].(string); cpu != "" {
		q, e := resource.ParseQuantity(cpu)
		if e != nil {
			return 0, 0, fmt.Errorf("cpu %q: %w", cpu, e)
		}
		milliCPU = q.MilliValue()
	}
	if mem, _ := rr["memory"].(string); mem != "" {
		q, e := resource.ParseQuantity(mem)
		if e != nil {
			return 0, 0, fmt.Errorf("memory %q: %w", mem, e)
		}
		memoryBytes = q.Value()
	}
	return milliCPU, memoryBytes, nil
}
