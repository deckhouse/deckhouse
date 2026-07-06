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

// ProviderInput is the native input for built-in provider preparators.
// Unlike proto.PrepareInput, it avoids serialization round-trips:
// ProviderClusterConfig stays as json.RawMessage and CloudProviderVars
// is already parsed.
type ProviderInput struct {
	ProviderName          string
	ClusterPrefix         string
	Layout                string
	Operation             string
	ProviderClusterConfig map[string]json.RawMessage
	CloudProviderVars     *proto.CloudProviderVars
}

type MetaConfigPreparator interface {
	Validate(ctx context.Context, input ProviderInput) error
	Prepare(ctx context.Context, input ProviderInput) (proto.PrepareResult, error)
}

// MetaConfigPreparatorProvider selects a MetaConfigPreparator for the given
// provider. downloadRootDir is the directory where provider images have been
// unpacked; external preparators look for their binary there.
type MetaConfigPreparatorProvider func(ctx context.Context, provider, downloadRootDir string) MetaConfigPreparator

type dummyPreparator struct{}

func DummyPreparatorProvider() MetaConfigPreparatorProvider {
	return func(_ context.Context, provider, _ string) MetaConfigPreparator {
		return &dummyPreparator{}
	}
}

func (p *dummyPreparator) Validate(_ context.Context, _ ProviderInput) error {
	return nil
}

func (p *dummyPreparator) Prepare(_ context.Context, _ ProviderInput) (proto.PrepareResult, error) {
	return proto.PrepareResult{}, nil
}
