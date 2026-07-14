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

package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

// PayloadHasClusterConfiguration scans documents structurally (no schema
// validation) and reports whether any ClusterConfiguration is present.
// Validators that only make sense for cluster configs use this as a
// pre-flight to avoid surfacing duplicates of Layer-1 errors on
// resource-only payloads.
func PayloadHasClusterConfiguration(payload string) bool {
	for _, doc := range input.YAMLSplitRegexp.Split(strings.TrimSpace(payload), -1) {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}
		var index SchemaIndex
		if err := yaml.Unmarshal([]byte(doc), &index); err != nil {
			continue
		}
		if index.Kind == ClusterConfigurationKind {
			return true
		}
	}
	return false
}

// ParseClusterPayload extracts cluster intent (ClusterConfiguration,
// *ClusterConfiguration, ModuleConfigs) without running OpenAPI validation
// or the heavyweight bootstrap-time Prepare (registry init, preparator
// hooks). It reuses parseDocument via ValidateOptionSkipSchemaValidation so
// the per-kind switch stays single-sourced.
//
// Used by domain analyzers (CNI mismatch, future ones) that need a partial
// MetaConfig to reason about user intent. Schema validation of the same
// payload is expected to happen separately in the schema-validation pass.
func ParseClusterPayload(ctx context.Context, payload string) (*MetaConfig, error) {
	meta := &MetaConfig{}
	for _, doc := range input.YAMLSplitRegexp.Split(strings.TrimSpace(payload), -1) {
		if _, err := parseDocument(ctx, doc, meta, nil, ValidateOptionSkipSchemaValidation(true)); err != nil {
			return nil, fmt.Errorf("parse cluster payload: %w", err)
		}
	}
	if meta.ClusterConfig == nil {
		return nil, fmt.Errorf("no ClusterConfiguration in payload")
	}

	var clusterType string
	if raw, ok := meta.ClusterConfig["clusterType"]; ok {
		_ = json.Unmarshal(raw, &clusterType)
	}
	meta.ClusterType = clusterType

	if clusterType == CloudClusterType {
		var cloud ClusterConfigCloudSpec
		if raw, ok := meta.ClusterConfig["cloud"]; ok {
			_ = json.Unmarshal(raw, &cloud)
		}
		meta.ProviderName = strings.ToLower(cloud.Provider)
		meta.OriginalProviderName = cloud.Provider
	}
	return meta, nil
}
