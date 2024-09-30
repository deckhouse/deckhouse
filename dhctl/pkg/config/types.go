// Copyright 2021 Flant JSC
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

package config

import "fmt"

const (
	CloudClusterType  = "Cloud"
	StaticClusterType = "Static"
)

type SchemaIndex struct {
	Kind    string `json:"kind"`
	Version string `json:"apiVersion"`
}

func (i *SchemaIndex) IsValid() bool {
	return i.Kind != "" && i.Version != ""
}

func (i *SchemaIndex) String() string {
	return fmt.Sprintf("%s, %s", i.Kind, i.Version)
}

type OpenAPISchema struct {
	Kind     string                 `json:"kind"`
	Versions []OpenAPISchemaVersion `json:"apiVersions"`
}

type OpenAPISchemaVersion struct {
	Version string      `json:"apiVersion"`
	Schema  interface{} `json:"openAPISpec"`
}

type ClusterConfigCloudSpec struct {
	Provider string `json:"provider"`
	Prefix   string `json:"prefix"`
}

type MasterNodeGroupSpec struct {
	Replicas int `json:"replicas"`
}

type YandexMasterNodeGroupSpec struct {
	Replicas      int `json:"replicas"`
	InstanceClass struct {
		ExternalIPAddresses []string `json:"externalIPAddresses"`
	} `json:"instanceClass"`
}

type YandexNodeGroupSpec struct {
	Name          string `json:"name"`
	Replicas      int    `json:"replicas"`
	InstanceClass struct {
		ExternalIPAddresses []string `json:"externalIPAddresses"`
	} `json:"instanceClass"`
}

type TerraNodeGroupSpec struct {
	Replicas     int                    `json:"replicas"`
	Name         string                 `json:"name"`
	NodeTemplate map[string]interface{} `json:"nodeTemplate"`
}

type DeckhouseClusterConfig struct {
	ReleaseChannel    string                 `json:"releaseChannel,omitempty"`
	DevBranch         string                 `json:"devBranch,omitempty"`
	Bundle            string                 `json:"bundle,omitempty"`
	LogLevel          string                 `json:"logLevel,omitempty"`
	ImagesRepo        string                 `json:"imagesRepo"`
	RegistryDockerCfg string                 `json:"registryDockerCfg,omitempty"`
	RegistryCA        string                 `json:"registryCA,omitempty"`
	RegistryScheme    string                 `json:"registryScheme,omitempty"`
	ConfigOverrides   map[string]interface{} `json:"configOverrides"` // Deprecated
}
