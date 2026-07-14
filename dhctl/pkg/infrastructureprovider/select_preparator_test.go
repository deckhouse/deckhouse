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
)

// In-tree providers without a dedicated preparator (gcp, aws, azure, ...)
// must fall back to the lightweight prefix-only preparator instead of
// demanding an external validator binary.
func TestSelectPreparatorInTreeFallback(t *testing.T) {
	orig := providerBundledInCandi
	providerBundledInCandi = func(string) bool { return true }
	t.Cleanup(func() { providerBundledInCandi = orig })

	p := selectPreparator(context.Background(), "gcp", t.TempDir())
	require.IsType(t, &inTreeDefaultPreparator{}, p)
	require.NoError(t, p.Validate(context.Background(), config.ProviderInput{ClusterPrefix: "ok-prefix", ProviderName: "gcp"}))
}

func TestSelectPreparatorExternalMissingValidator(t *testing.T) {
	orig := providerBundledInCandi
	providerBundledInCandi = func(string) bool { return false }
	t.Cleanup(func() { providerBundledInCandi = orig })

	p := selectPreparator(context.Background(), "dvp", t.TempDir())
	require.IsType(t, &missingExternalValidatorPreparator{}, p)
}
