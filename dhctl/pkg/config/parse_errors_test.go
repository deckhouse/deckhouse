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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// writeConfig puts a configuration on disk the way an operator hands one to dhctl.
func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// TestParseConfigReportsEveryBadDocument: three mistakes in one file used to cost three runs, each
// finding one — and on a cloud cluster a run that gets past this creates infrastructure.
func TestParseConfigReportsEveryBadDocument(t *testing.T) {
	path := writeConfig(t, `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.33"
clusterDomain: cluster.local
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: 42
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: "not a number"
  enabled: true
`)

	_, err := ParseConfig(t.Context(), []string{path}, DummyValidatorProvider(), &options.New().Global)
	require.Error(t, err)

	text := err.Error()
	for _, want := range []string{"imagesRepo", "version"} {
		assert.Containsf(t, text, want, "every bad document must be reported, got:\n%s", text)
	}
}

// TestParseConfigPrintsTheDocumentOnce: it used to be printed twice — once raw by the validator
// and once with line numbers by the caller — so a three-document config produced screens of YAML
// with the one useful copy somewhere in the middle.
func TestParseConfigPrintsTheDocumentOnce(t *testing.T) {
	const marker = "imagesRepo: 42"

	path := writeConfig(t, `
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  `+marker+`
`)

	_, err := ParseConfig(t.Context(), []string{path}, DummyValidatorProvider(), &options.New().Global)
	require.Error(t, err)

	text := err.Error()
	assert.Equalf(t, 1, strings.Count(text, marker),
		"the document must appear once, and with line numbers; got:\n%s", text)
	assert.Containsf(t, text, "1\tapiVersion", "the copy that is kept is the numbered one; got:\n%s", text)
}

// TestPatternErrorShowsAnExample: go-openapi states the rule as the regular expression itself,
// which is exact and useless — the reader has to decode it to find out that a prefix length is
// missing. The schemas carry x-examples for this, and one of them says the same thing in a form
// that can be copied.
func TestPatternErrorShowsAnExample(t *testing.T) {
	path := writeConfig(t, `
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
- 10.244.0.0
`)

	_, err := ParseConfig(t.Context(), []string{path}, DummyValidatorProvider(), &options.New().Global)
	require.Error(t, err)

	text := err.Error()
	assert.Containsf(t, text, "should look like", "the message must show a value, got:\n%s", text)
	assert.NotContainsf(t, text, `[1-9][0-9]`, "the regular expression must not be the message, got:\n%s", text)
}

// TestNonPatternErrorsAreLeftAlone: "must be of type string" already says what is wrong, and
// rewriting every message would lose the ones that were fine.
func TestNonPatternErrorsAreLeftAlone(t *testing.T) {
	path := writeConfig(t, `
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: 42
`)

	_, err := ParseConfig(t.Context(), []string{path}, DummyValidatorProvider(), &options.New().Global)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be of type string")
}
