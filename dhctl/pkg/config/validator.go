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

// ProviderInput is the native input for provider validators. Unlike a
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

// MetaConfigValidator checks a provider's configuration. Validation is the only
// thing every provider shares: a provider that additionally has to rewrite the
// parsed configuration (only vcd does) implements the optional patcher method
// documented in validateProviderConfig.
type MetaConfigValidator interface {
	Validate(ctx context.Context, input ProviderInput) error
}

// MetaConfigValidatorProvider selects a MetaConfigValidator for the given
// provider.
type MetaConfigValidatorProvider func(ctx context.Context, provider string) MetaConfigValidator

type dummyValidator struct{}

// DummyValidatorProvider validates nothing. It fits providers whose config is
// checked elsewhere: in-tree providers by their candi OpenAPI schema, external
// ones (e.g. DVP) by the cloud-provider admission webhook.
func DummyValidatorProvider() MetaConfigValidatorProvider {
	return func(_ context.Context, _ string) MetaConfigValidator {
		return &dummyValidator{}
	}
}

func (p *dummyValidator) Validate(_ context.Context, _ ProviderInput) error {
	return nil
}
