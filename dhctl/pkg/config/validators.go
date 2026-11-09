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
	"errors"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

type validator func(
	ctx context.Context,
	payload string,
	store *SchemaStore,
	opts ...ValidateOption,
) *ValidationError

// ResourcesPipeline runs validators in layers, each surfacing errors
// independently:
//   - validateUserResources  — structural (apiVersion/kind/CRD)
//   - validateConfigSchema   — schema (OpenAPI) for the whole payload
//   - validateCNIBootstrap   — domain (CNI mismatch vs cni-bootstrap.yml)
//   - validatePreviousRegistryModes — domain (Proxy/Local of the previous registry implementation)
//
// New domain validators (CSI,Cloud) plug in as additional
// Layer-3 entries: they filter the payload to their concern and operate on
// top of a partial MetaConfig from ParseClusterPayload.
var ResourcesPipeline = []validator{
	validateUserResources,
	validateConfigSchema,
	validateCNIBootstrap,
	validatePreviousRegistryModes,
}

func validateUserResources(_ context.Context, payload string, _ *SchemaStore, opts ...ValidateOption) *ValidationError {
	err := ValidateResources(payload, opts...)
	if err == nil {
		return nil
	}
	if ve, ok := errors.AsType[*ValidationError](err); ok {
		return ve
	}
	out := &ValidationError{}
	out.Append(ErrKindValidationFailed, Error{Messages: []string{err.Error()}})
	return out
}

// validateConfigSchema runs full ParseConfigFromData with multi-error
// accumulation. It surfaces every OpenAPI / structural problem found across
// ClusterConfiguration, *ClusterConfiguration, InitConfiguration and every
// ModuleConfig in the payload in a single pass. Resource-only payloads
// (no ClusterConfiguration) are skipped — Layer 1 already covers those.
func validateConfigSchema(ctx context.Context, payload string, _ *SchemaStore, opts ...ValidateOption) *ValidationError {
	if !PayloadHasClusterConfiguration(payload) {
		return nil
	}
	opts = append(opts, ValidateOptionCollectAllErrors(true))
	if _, err := ParseConfigFromData(ctx, payload, DummyValidatorProvider(), nil, opts...); err != nil {
		if ve, ok := errors.AsType[*ValidationError](err); ok {
			return ve
		}
		out := &ValidationError{}
		out.Append(ErrKindValidationFailed, Error{Messages: []string{err.Error()}})
		return out
	}
	return nil
}

// validatePreviousRegistryModes refuses the `Proxy` and `Local` registry modes of the previous
// implementation in every deckhouse ModuleConfig of a payload that carries no
// ClusterConfiguration: the resources of an installation, validated on their own. The installation
// would refuse them too, but only once it is started, and this is where commander checks the
// resources before that.
//
// A payload with a ClusterConfiguration is left to validateConfigSchema: its MetaConfig.Prepare
// refuses the same modes in every copy of the ModuleConfig (refusePreviousRegistryModes), and
// checking here too would report the same document twice. Documents that do not even index-parse
// are skipped: their structural problems belong to validateUserResources.
func validatePreviousRegistryModes(_ context.Context, payload string, _ *SchemaStore, _ ...ValidateOption) *ValidationError {
	if PayloadHasClusterConfiguration(payload) {
		return nil
	}

	out := &ValidationError{}
	for i, doc := range input.YAMLSplitRegexp.Split(strings.TrimSpace(payload), -1) {
		if doc == "" {
			continue
		}

		var index namedIndex
		if err := yaml.Unmarshal([]byte(doc), &index); err != nil {
			continue
		}
		if index.Kind != ModuleConfigKind || index.Metadata.Name != "deckhouse" {
			continue
		}

		if err := registry.RefusePreviousImplementationModesInModuleConfig([]byte(doc)); err != nil {
			out.Append(ErrKindValidationFailed, Error{
				Index:    new(i),
				Group:    ModuleConfigGroup,
				Version:  ModuleConfigVersion,
				Kind:     ModuleConfigKind,
				Name:     index.Metadata.Name,
				Messages: []string{err.Error()},
			})
		}
	}

	if len(out.Errors) == 0 {
		return nil
	}

	return out
}
