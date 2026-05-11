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

import "github.com/deckhouse/deckhouse/dhctl/pkg/config"

// prepared is the value produced by prepareDestroyPhase and threaded
// through the rest of the destroy pipeline. The chosen infraDestroyer
// carries its own internal state across phases (e.g. static credentials
// set up in deleteResources and reused in destroyInfra).
type prepared struct {
	metaConfig  *config.MetaConfig
	clusterType string
	destroyer   infraDestroyer
}
