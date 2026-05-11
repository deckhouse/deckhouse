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

import (
	"context"

	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/deckhouse"
)

// finalizeResourcesPhase persists "deckhouse resources deleted" into the
// state cache so a subsequent attempt resumes from the right point.
type finalizeResourcesPhase struct {
	d8Destroyer *deckhouse.Destroyer
}

func (p finalizeResourcesPhase) Name() string { return "finalize-resources" }

func (p finalizeResourcesPhase) Run(ctx context.Context, _ *destroyState) error {
	return p.d8Destroyer.Finalize(ctx)
}
