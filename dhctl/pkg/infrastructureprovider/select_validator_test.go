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

package infrastructureprovider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vcd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/yandex"
)

// In-tree providers without a dedicated preparator (gcp, aws, azure, ...)
// must fall back to the lightweight prefix-only preparator instead of
// demanding an external validator binary.
func TestSelectValidatorInTreeFallback(t *testing.T) {
	orig := providerBundledInCandi
	providerBundledInCandi = func(string) bool { return true }
	t.Cleanup(func() { providerBundledInCandi = orig })

	p := selectValidator(context.Background(), "gcp", t.TempDir())
	require.IsType(t, &inTreeDefaultValidator{}, p)
	require.NoError(t, p.Validate(context.Background(), config.ProviderInput{ClusterPrefix: "ok-prefix", ProviderName: "gcp"}))
}

func TestSelectValidatorExternalMissingValidator(t *testing.T) {
	orig := providerBundledInCandi
	providerBundledInCandi = func(string) bool { return false }
	t.Cleanup(func() { providerBundledInCandi = orig })

	p := selectValidator(context.Background(), "dvp", t.TempDir())
	require.IsType(t, &missingExternalValidator{}, p)
}

// The vcd legacyMode patch reaches config through an optional-method check, so
// nothing at compile time ties vcd's real type to the contract config expects:
// rename the method or switch its receiver and the patch silently stops
// applying. Pin the real selector output against that contract.
func TestSelectValidatorVCDSatisfiesPatcherContract(t *testing.T) {
	type providerClusterConfigPatcher interface {
		PatchProviderClusterConfig(ctx context.Context, input config.ProviderInput) (map[string]any, error)
	}

	v := selectValidator(context.Background(), vcd.ProviderName, "")
	require.Implements(t, (*providerClusterConfigPatcher)(nil), v, "vcd must keep patching providerClusterConfiguration (legacyMode)")

	other := selectValidator(context.Background(), yandex.ProviderName, "")
	require.NotImplements(t, (*providerClusterConfigPatcher)(nil), other, "only vcd may patch the parsed config")
}
