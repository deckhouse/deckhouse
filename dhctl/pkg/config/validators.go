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
//
// New domain validators (CSI,Cloud) plug in as additional
// Layer-3 entries: they filter the payload to their concern and operate on
// top of a partial MetaConfig from ParseClusterPayload.
var ResourcesPipeline = []validator{
	validateUserResources,
	validateConfigSchema,
	validateCNIBootstrap,
}

func validateUserResources(_ context.Context, payload string, _ *SchemaStore, opts ...ValidateOption) *ValidationError {
	err := ValidateResources(payload, opts...)
	if err == nil {
		return nil
	}
	var ve *ValidationError
	if errors.As(err, &ve) {
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
	if _, err := ParseConfigFromData(ctx, payload, DummyPreparatorProvider(), nil, opts...); err != nil {
		var ve *ValidationError
		if errors.As(err, &ve) {
			return ve
		}
		out := &ValidationError{}
		out.Append(ErrKindValidationFailed, Error{Messages: []string{err.Error()}})
		return out
	}
	return nil
}
