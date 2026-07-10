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
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// captureSlog installs a buffer-backed slog logger as the process default so
// that warnDeprecatedFields, which logs via dhlog.FromContext(context.Background())
// under a plain context.Background(), is captured. These tests do not run in
// parallel, so mutating the global slog default is safe.
func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(dhlog.NewBufferLogger(&buf))
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
			wantContains:   []string{"DEPRECATED", "oldOption", "TestDeprecationKind"},
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
			wantContains: []string{"DEPRECATED", "oldOption", "TestDeprecationKind", "my-resource"},
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

			_, err := newStore.Validate(new([]byte(tt.content)))
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
