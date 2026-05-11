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

package destroy

import "context"

// populateMetaConfigPhase loads the cluster meta config either from the
// commander-supplied payload (commander mode) or by re-hydrating it from
// the state cache via the terra state loader.
//
// Reads: state.configPreparator, state.directoryConfig.
// Writes: state.metaConfig.
type populateMetaConfigPhase struct{}

func (populateMetaConfigPhase) Name() string { return "populate-meta-config" }

func (populateMetaConfigPhase) Run(ctx context.Context, s *destroyState) error {
	mc, err := s.configPreparator.PopulateMetaConfig(ctx, s.directoryConfig)
	if err != nil {
		return err
	}
	s.metaConfig = mc
	return nil
}
