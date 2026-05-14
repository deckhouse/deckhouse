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

// ControlPlaneTemplateConfig is the data passed to control-plane template rendering.
//
// Settings holds ModuleConfig control-plane-manager spec.settings (authoritative source).
// ClusterConfiguration holds legacy ClusterConfiguration data (fallback during migration).
// Templates choose the source explicitly: `coalesce .settings.field .clusterConfiguration.field`.
// ToMap is the only boundary with the Go template engine.
type ControlPlaneTemplateConfig struct {
	RunType    string                 `json:"runType"`
	NodeIP     string                 `json:"nodeIP"`
	NodeName   string                 `json:"nodeName"`
	Registry   map[string]interface{} `json:"registry"`
	Images     map[string]interface{} `json:"images"`
	VersionMap map[string]interface{} `json:"-"`

	Settings             map[string]interface{} `json:"settings"`
	ClusterConfiguration map[string]interface{} `json:"clusterConfiguration"`
}

// ToMap is the only entry point into the Go template engine. It converts the typed struct
// to a flat map so templates can access all fields uniformly. VersionMap keys (k8s version
// data, image digests, etc.) are merged into the root. Explicit fields win over VersionMap
// keys with the same name.
func (c *ControlPlaneTemplateConfig) ToMap() map[string]interface{} {
	m := make(map[string]interface{})
	for k, v := range c.VersionMap {
		m[k] = v
	}
	m["runType"] = c.RunType
	m["nodeIP"] = c.NodeIP
	m["nodeName"] = c.NodeName
	m["registry"] = c.Registry
	m["images"] = c.Images
	m["settings"] = c.Settings
	m["clusterConfiguration"] = c.ClusterConfiguration
	return m
}
