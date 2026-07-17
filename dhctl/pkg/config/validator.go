// Copyright 2025 Flant JSC
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

	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
)

// ProviderInput is the native input for provider validation. Unlike a
// serialized form, ProviderClusterConfig stays as json.RawMessage and
// CloudProviderVars is already parsed.
type ProviderInput struct {
	ProviderName          string
	ClusterPrefix         string
	Layout                string
	Operation             string
	ProviderClusterConfig map[string]json.RawMessage
	CloudProviderVars     *proto.CloudProviderVars
}

// ProviderValidateFunc checks a provider's configuration. Validation never
// mutates anything; the single provider-side rewrite that exists (vcd's
// legacyMode) lives entirely in the infrastructure layer.
type ProviderValidateFunc func(ctx context.Context, input ProviderInput) error

// MetaConfigValidatorProvider selects the validate function for the given
// provider; nil means the provider has nothing to check. downloadRootDir is
// where provider bundles are unpacked; an external provider's validator binary
// is looked up there.
type MetaConfigValidatorProvider func(ctx context.Context, provider, downloadRootDir string) ProviderValidateFunc

// DummyValidatorProvider validates nothing. Use it where provider validation is
// deliberately out of scope (static clusters, config parses that only need the
// typed fields), not as a fallback for a provider that has a validator.
func DummyValidatorProvider() MetaConfigValidatorProvider {
	return func(_ context.Context, _, _ string) ProviderValidateFunc {
		return nil
	}
}
