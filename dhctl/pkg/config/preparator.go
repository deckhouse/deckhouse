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

import "context"

type MetaConfigPreparator interface {
	Validate(ctx context.Context, metaConfig *MetaConfig) error
	Prepare(ctx context.Context, metaConfig *MetaConfig) error
}

type MetaConfigPreparatorProvider func(provider string) MetaConfigPreparator

type dummyPreparator struct{}

func DummyPreparatorProvider() MetaConfigPreparatorProvider {
	return func(provider string) MetaConfigPreparator {
		return &dummyPreparator{}
	}
}

func (p *dummyPreparator) Validate(_ context.Context, _ *MetaConfig) error {
	return nil
}

func (p *dummyPreparator) Prepare(_ context.Context, _ *MetaConfig) error {
	return nil
}
