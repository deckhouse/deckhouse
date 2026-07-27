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

import (
	"context"
	"fmt"
)

const (
	CloudClusterType  = "Cloud"
	StaticClusterType = "Static"
)

// ProviderRequiresClusterConfig reports whether the given cloud provider must
// have a <Provider>ClusterConfiguration section. Derived from the environment
// instead of a hardcoded provider list, so dhctl needs no knowledge of new
// providers: a provider whose schemas ship in the image's candi (in-tree)
// follows the legacy contract, anything else (external OCI-bundle providers,
// e.g. DVP) may be configured via ModuleConfig alone. The section's content is
// enforced by the provider's own OpenAPI schema (required: [layout,
// masterNodeGroup, ...]), and where candi is absent that schema is absent too,
// so such a section could not have been parsed in the first place.
func ProviderRequiresClusterConfig(providerName string) bool {
	return ProviderBundledInCandi(providerName, nil)
}

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
	Version string `json:"apiVersion"`
	Schema  any    `json:"openAPISpec"`
}

type ClusterConfigCloudSpec struct {
	Provider string `json:"provider"`
	Prefix   string `json:"prefix"`
}

type MasterNodeGroupSpec struct {
	Replicas int `json:"replicas"`
}

type TerraNodeGroupSpec struct {
	Replicas     int            `json:"replicas"`
	Name         string         `json:"name"`
	NodeTemplate map[string]any `json:"nodeTemplate"`
}

type DeckhouseClusterConfig struct {
	ReleaseChannel    string         `json:"releaseChannel,omitempty"` // Deprecated
	DevBranch         string         `json:"devBranch,omitempty"`
	Bundle            string         `json:"bundle,omitempty"`   // Deprecated
	LogLevel          string         `json:"logLevel,omitempty"` // Deprecated
	ImagesRepo        string         `json:"imagesRepo"`
	RegistryDockerCfg string         `json:"registryDockerCfg,omitempty"`
	RegistryCA        string         `json:"registryCA,omitempty"`
	RegistryScheme    string         `json:"registryScheme,omitempty"`
	ConfigOverrides   map[string]any `json:"configOverrides"` // Deprecated
}

type ByClusterType[T any] interface {
	Cloud(context.Context, *MetaConfig) (T, error)
	Static(context.Context, *MetaConfig) (T, error)
	Incorrect(context.Context, *MetaConfig) (T, error)
}

func DoByClusterType[T any](ctx context.Context, metaConfig *MetaConfig, actor ByClusterType[T]) (T, error) {
	switch metaConfig.ClusterType {
	case CloudClusterType:
		return actor.Cloud(ctx, metaConfig)
	case StaticClusterType:
		return actor.Static(ctx, metaConfig)
	default:
		return actor.Incorrect(ctx, metaConfig)
	}
}

func UnsupportedClusterTypeErr(metaConfig *MetaConfig) error {
	return fmt.Errorf("Unsupported cluster type: '%s'", metaConfig.ClusterType)
}
