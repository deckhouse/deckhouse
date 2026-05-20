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

package log_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// TestLogOutput_MarshalJSON_ControlBytes guards the JSON encoder against
// callers that emit C0 control bytes (RFC 8259 §7 requires them all to be
// escaped). The original bug surfaced when nelm's progress tracker pushed
// SGR-coloured strings into the logger and json.Marshal returned
// "invalid character '\x1b' in string literal".
func TestLogOutput_MarshalJSON_ControlBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		msg  string
		// substring expected somewhere in the produced JSON
		wantContains string
	}{
		{
			name:         "ansi-sgr-escape",
			msg:          "\x1b[36mABSENT\x1b[0m",
			wantContains: `\u001b[36mABSENT\u001b[0m`,
		},
		{
			name:         "vertical-tab",
			msg:          "before\vafter",
			wantContains: `before\u000bafter`,
		},
		{
			name:         "bell",
			msg:          "alert\aend",
			wantContains: `alert\u0007end`,
		},
		{
			name:         "nul-byte",
			msg:          "x\x00y",
			wantContains: `x\u0000y`,
		},
		// Named-escape forms must keep their short JSON representation.
		{
			name:         "newline-named-escape",
			msg:          "line1\nline2",
			wantContains: `line1\nline2`,
		},
		{
			name:         "tab-named-escape",
			msg:          "a\tb",
			wantContains: `a\tb`,
		},
		{
			name:         "carriage-return-named-escape",
			msg:          "a\rb",
			wantContains: `a\rb`,
		},
		{
			name:         "form-feed-uXXXX",
			msg:          "a\fb",
			wantContains: `a\u000cb`,
		},
		{
			name:         "backspace-uXXXX",
			msg:          "a\bb",
			wantContains: `a\u0008b`,
		},
		// Non-control special characters keep their existing handling.
		{
			name:         "double-quote",
			msg:          `say "hi"`,
			wantContains: `say \"hi\"`,
		},
		{
			name:         "backslash",
			msg:          `a\b`,
			wantContains: `a\\b`,
		},
		{
			// Render escapes '<' as \u003c; json.Marshal then HTML-compacts
			// the returned bytes and also encodes '>' as \u003e. Assert both
			// to be explicit about the final wire form.
			name:         "html-less-than",
			msg:          "<script>",
			wantContains: `\u003cscript\u003e`,
		},
		// No escapable bytes → fast path, value passes through unchanged.
		{
			name:         "plain-ascii-fast-path",
			msg:          "hello world",
			wantContains: `"msg":"hello world"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out := &log.LogOutput{
				Level:   "info",
				Name:    "test.logger",
				Message: tc.msg,
				Time:    "2026-05-19T18:16:47Z",
			}

			b, err := json.Marshal(out)
			require.NoError(t, err, "MarshalJSON should produce valid JSON for any string content")

			// The produced bytes must be parseable by the stdlib encoder.
			var probe map[string]any
			require.NoError(t, json.Unmarshal(b, &probe), "encoder output must be valid JSON; got %s", string(b))

			// Sanity: the message round-trips back to the original string.
			assert.Equal(t, tc.msg, probe["msg"], "msg should round-trip unchanged")

			assert.Contains(t, string(b), tc.wantContains, "encoded JSON should contain %q", tc.wantContains)
		})
	}
}

// TestLogOutput_MarshalJSON_NelmPayload reproduces the exact failure mode from
// the production log: a nelm-styled progress table containing ESC SGR codes
// and newlines. Before the fix this returned an error; after the fix the
// resulting bytes must be valid JSON and round-trip.
func TestLogOutput_MarshalJSON_NelmPayload(t *testing.T) {
	t.Parallel()

	msg := "\x1b[1mRESOURCE (→ABSENT)\x1b[0m  \x1b[1mSTATE\x1b[0m\n" +
		"\x1b[36mClusterObservabilityDashboard\x1b[0m/foo  \x1b[32mABSENT\x1b[0m\n" +
		"\x1b[36mClusterObservabilityMetricsRulesGroup\x1b[0m/bar  \x1b[33mWAITING\x1b[0m"

	out := &log.LogOutput{
		Level:   "info",
		Name:    "addon-operator.module-manager.helm-module.helm-client.nelm",
		Message: msg,
		Time:    "2026-05-19T18:16:47Z",
		Fields:  map[string]any{"module": "deckhouse"},
	}

	b, err := json.Marshal(out)
	require.NoError(t, err)

	var probe map[string]any
	require.NoError(t, json.Unmarshal(b, &probe), "encoder output must be valid JSON; got %s", string(b))

	assert.Equal(t, msg, probe["msg"])
	assert.Equal(t, "deckhouse", probe["module"])
	// Every ESC byte must be escaped as \u001b.
	assert.False(t, strings.ContainsRune(string(b), 0x1b), "no raw ESC bytes should remain in JSON output")
}

// TestLogOutput_Text_PreservesNamedEscapes ensures the shared Render path used
// by the text handler keeps its existing behaviour for the named-escape forms
// it has always handled, including the newly-routed control bytes.
func TestLogOutput_Text_HandlesControlBytes(t *testing.T) {
	t.Parallel()

	out := &log.LogOutput{
		Level:   "info",
		Name:    "test.logger",
		Message: "esc=\x1b reset",
		Time:    "2026-05-19T18:16:47Z",
	}

	b, err := out.Text()
	require.NoError(t, err)

	// The text encoder uses the same Render.string path, so the ESC must be
	// emitted as a \u001b escape rather than a raw byte.
	assert.Contains(t, string(b), `\u001b`)
	assert.False(t, strings.ContainsRune(string(b), 0x1b), "no raw ESC bytes should remain in text output")
}
