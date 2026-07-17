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

// In-tree providers without a dedicated validator (gcp, aws, azure, ...)
// must fall back to the lightweight prefix-only check instead of demanding an
// external validator binary.
func TestSelectValidatorInTreeFallback(t *testing.T) {
	orig := providerBundledInCandi
	providerBundledInCandi = func(string) bool { return true }
	t.Cleanup(func() { providerBundledInCandi = orig })

	h := selectValidator(context.Background(), "gcp", t.TempDir())
	require.Nil(t, h.Patch)
	require.NoError(t, h.Validate(context.Background(), config.ProviderInput{ClusterPrefix: "ok-prefix", ProviderName: "gcp"}))
	require.Error(t, h.Validate(context.Background(), config.ProviderInput{ProviderName: "gcp"}), "empty prefix must fail")
}

func TestSelectValidatorExternalMissingValidator(t *testing.T) {
	orig := providerBundledInCandi
	providerBundledInCandi = func(string) bool { return false }
	t.Cleanup(func() { providerBundledInCandi = orig })

	h := selectValidator(context.Background(), "dvp", t.TempDir())
	err := h.Validate(context.Background(), config.ProviderInput{})
	require.ErrorContains(t, err, "external validator for provider \"dvp\" not found")
}

// Only vcd patches the parsed providerClusterConfiguration (legacyMode); for
// everyone else the Patch handler must stay nil.
func TestSelectValidatorOnlyVCDPatches(t *testing.T) {
	require.NotNil(t, selectValidator(context.Background(), vcd.ProviderName, "").Patch)
	require.Nil(t, selectValidator(context.Background(), yandex.ProviderName, "").Patch)
}
