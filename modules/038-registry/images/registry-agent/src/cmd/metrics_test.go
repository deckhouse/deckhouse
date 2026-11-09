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

package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agentmetrics "github.com/deckhouse/registry-agent/internal/metrics"
)

// TestTheTextfileCarriesNothingButTheAgentsOwnFamilies asserts the split main actually wires.
//
// The node exporter merges this file into its own answer, and it reports go_* and process_* of
// its own. One registry for both readers would put those names into the file a second time,
// with other help and other values, and the node exporter would answer its scrape with an
// inconsistency: the agent's series lost at best, the node's whole scrape failing at worst.
// Every line of the file has to belong to the agent.
func TestTheTextfileCarriesNothingButTheAgentsOwnFamilies(t *testing.T) {
	served, ownOnly, collected := buildMetrics()

	// Something in every family, so that an empty vector cannot make the file trivially clean.
	collected.Requests.WithLabelValues("managed", agentmetrics.ResultServed).Inc()
	collected.Failovers.WithLabelValues("storage", agentmetrics.ReasonUnreachable).Inc()
	collected.Tokens.WithLabelValues(agentmetrics.TokenObtained).Inc()
	collected.LayoutOrigin.WithLabelValues("api").Set(1)

	path := filepath.Join(t.TempDir(), "registry-agent.prom")
	textfile := &agentmetrics.Textfile{
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Path:     path,
		Gatherer: ownOnly,
	}
	require.NoError(t, textfile.Write())

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	families := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		name := line
		if rest, ok := strings.CutPrefix(line, "# HELP "); ok {
			name = rest
		} else if rest, ok := strings.CutPrefix(line, "# TYPE "); ok {
			name = rest
		}
		require.True(t, strings.HasPrefix(name, "d8_registry_agent_"),
			"the textfile carries a line that is not the agent's: %q", line)
		families[strings.FieldsFunc(name, func(r rune) bool { return r == ' ' || r == '{' })[0]] = true
	}
	assert.Equal(t, map[string]bool{
		"d8_registry_agent_requests_total":        true,
		"d8_registry_agent_failovers_total":       true,
		"d8_registry_agent_token_exchanges_total": true,
		"d8_registry_agent_layout_origin":         true,
	}, families)

	// The loopback listener keeps what it had: someone on the node still gets the runtime along
	// with the agent's families.
	gathered, err := served.Gather()
	require.NoError(t, err)
	names := map[string]bool{}
	for _, family := range gathered {
		names[family.GetName()] = true
	}
	assert.True(t, names["go_goroutines"], "the loopback listener lost the Go runtime")
	assert.True(t, names["d8_registry_agent_requests_total"], "the loopback listener lost the agent's own families")
}
