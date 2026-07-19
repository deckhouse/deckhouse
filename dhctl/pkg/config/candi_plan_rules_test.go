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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdir"
)

// The download dir is reused across dhctl runs: bootstrap installs an external
// provider bundle, which writes a single-provider terraform_versions.yml plus
// plan_rules.yml into candi. The next run re-extracts the multi-provider candi
// over the versions file but ships no plan rules, so a surviving plan_rules.yml
// would pair with a versions file it does not describe — the settings loader
// then refuses to start with "requires a single-provider bundle, got N
// providers" and check/converge cannot run at all.
func TestDropStalePlanRules(t *testing.T) {
	candiDir := t.TempDir()
	planRules := filepath.Join(candiDir, providerdir.PlanRulesFilename)

	require.NoError(t, os.WriteFile(planRules, []byte("vmResource: {}\n"), 0o644))
	require.NoError(t, dropStalePlanRules(candiDir))
	require.NoFileExists(t, planRules, "plan rules left by a previous bundle install must not survive a candi extract")

	// Nothing to drop is the normal case and must not fail.
	require.NoError(t, dropStalePlanRules(candiDir))
}
