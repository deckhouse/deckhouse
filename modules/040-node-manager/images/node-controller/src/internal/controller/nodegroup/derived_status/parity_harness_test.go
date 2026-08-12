/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package derived_status

import (
	"testing"

	"github.com/stretchr/testify/require"
	sigsyaml "sigs.k8s.io/yaml"
)

// The rewrite must not move a single byte of the published element: it is hashed into every node's
// configuration checksum and rendered into the name of an immutable machine template. This harness
// compares the serialized output of the frozen legacy builder against the current one across the
// whole corpus, so a divergence shows up as a diff rather than as a cluster-wide node roll.
//
// It is deleted together with the legacy builder once the rewrite lands.
func assertParity(t *testing.T, in ResolveInput, r Result) {
	t.Helper()

	legacy, err := sigsyaml.Marshal(legacyBuildNodeGroupBlob(in, r))
	require.NoError(t, err, "marshal legacy blob")

	rewritten, err := sigsyaml.Marshal(resolvedMap(in, r))
	require.NoError(t, err, "marshal rewritten element")

	require.Equal(t, string(legacy), string(rewritten))
}

func TestRewriteParityOnCorpus(t *testing.T) {
	for _, fixture := range nodeGroupCorpus() {
		t.Run(fixture.name, func(t *testing.T) {
			assertParity(t, fixture.input, fixture.result)
		})
	}
}
