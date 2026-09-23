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
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// captureSlog installs a rendering logger as the process default so that warnDeprecatedFields,
// which logs via dhlog.FromContext under a plain context.Background(), is captured. It renders
// rather than dumping raw records, so the assertions below read the badges and text a user sees
// instead of the attributes behind them. These tests do not run in parallel, so mutating the
// global slog default is safe.
func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(dhlog.NewStreamLogger(&buf))
	t.Cleanup(func() { slog.SetDefault(prev) })

	return &buf
}

func TestWarnDeprecatedFields(t *testing.T) {
	newStore := newSchemaStore(&options.New().Global, []string{"/tmp"})

	err := newStore.upload([]byte(`
kind: TestDeprecationKind
apiVersions:
- apiVersion: test
  openAPISpec:
    type: object
    properties:
      kind:
        type: string
      apiVersion:
        type: string
      oldOption:
        type: string
        x-doc-deprecated: true
        description: |
          Deprecated. Use newOption instead.
      newOption:
        type: string
      metadata:
        type: object
        properties:
          name:
            type: string
`))
	require.NoError(t, err)

	tests := map[string]struct {
		content        string
		wantWarning    bool
		wantContains   []string
		wantNotContain []string
	}{
		"deprecated field set": {
			content: `
kind: TestDeprecationKind
apiVersion: test
oldOption: legacy-value
`,
			wantWarning:    true,
			wantContains:   []string{"DEPRECATED TestDeprecationKind: oldOption"},
			wantNotContain: []string{"Deprecated. Use newOption instead."},
		},
		"deprecated field set with metadata name": {
			content: `
kind: TestDeprecationKind
apiVersion: test
metadata:
  name: my-resource
oldOption: legacy-value
`,
			wantWarning:  true,
			wantContains: []string{`DEPRECATED TestDeprecationKind "my-resource": oldOption`},
		},
		"deprecated field absent": {
			content: `
kind: TestDeprecationKind
apiVersion: test
newOption: current-value
`,
			wantWarning: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			buf := captureSlog(t)

			_, err := newStore.Validate(t.Context(), new([]byte(tt.content)))
			require.NoError(t, err)

			if !tt.wantWarning {
				require.NotContains(t, buf.String(), "DEPRECATED")
				return
			}

			for _, s := range tt.wantContains {
				require.Contains(t, buf.String(), s)
			}
			for _, s := range tt.wantNotContain {
				require.NotContains(t, buf.String(), s)
			}
		})
	}
}

// TestDeprecationsAreGroupedInOneReport covers the collection scope: several documents, each with
// deprecated options, must produce a single report - not a separator banner per field - and a
// document validated twice must not be listed twice.
func TestDeprecationsAreGroupedInOneReport(t *testing.T) {
	newStore := newSchemaStore(&options.New().Global, []string{"/tmp"})

	require.NoError(t, newStore.upload([]byte(`
kind: TestGroupedKind
apiVersions:
- apiVersion: test
  openAPISpec:
    type: object
    properties:
      kind:
        type: string
      apiVersion:
        type: string
      oldOption:
        type: string
        x-doc-deprecated: true
      alsoOld:
        type: string
        x-doc-deprecated: true
      metadata:
        type: object
        properties:
          name:
            type: string
`)))

	buf := captureSlog(t)

	ctx, flush := withDeprecationCollector(context.Background())

	first := []byte(`
kind: TestGroupedKind
apiVersion: test
oldOption: a
alsoOld: b
`)
	second := []byte(`
kind: TestGroupedKind
apiVersion: test
metadata:
  name: named
oldOption: c
`)

	for _, doc := range [][]byte{first, second, first} { // first is validated twice on purpose
		d := doc
		_, err := newStore.Validate(ctx, &d)
		require.NoError(t, err)
	}

	require.NotContains(t, buf.String(), "DEPRECATED", "nothing is reported before the scope is flushed")

	flush(ctx)

	out := buf.String()
	// One badge per option, deduplicated, and the explanation said once - not around every field.
	require.Equal(t, 3, strings.Count(out, "DEPRECATED "), "one badge per option:\n%s", out)
	require.Equal(t, 1, strings.Count(out, "Support for them will be removed"), "explained once:\n%s", out)
	require.Contains(t, out, "3 deprecated options are set")
	require.Contains(t, out, "DEPRECATED TestGroupedKind: alsoOld")
	require.Contains(t, out, "DEPRECATED TestGroupedKind: oldOption")
	require.Contains(t, out, "DEPRECATED TestGroupedKind \"named\": oldOption")

	// A second flush of an already-reported scope stays silent.
	buf.Reset()
	flush(ctx)
	require.NotContains(t, buf.String(), "DEPRECATED")
}
