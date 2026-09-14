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

package bootstrap

import (
	"bytes"
	"strings"
	"testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// TestPreflightListCoversEveryCheck is the guard on the discoverability the report promises: a
// failure prints `--preflight-skip-check=<name>`, and this is where the reader finds the rest of
// the names. A check missing here is a check nobody can find.
func TestPreflightListCoversEveryCheck(t *testing.T) {
	listed := map[string]struct{}{}
	for _, row := range preflightCheckRows() {
		listed[row.name] = struct{}{}
	}

	for _, name := range options.GeneratedChecks() {
		if _, ok := listed[name]; !ok {
			t.Errorf("check %q is accepted by --preflight-skip-check but is not listed by `dhctl preflight list`", name)
		}
	}
}

// TestPreflightListPrintsATable checks the shape an operator actually reads.
func TestPreflightListPrintsATable(t *testing.T) {
	var out bytes.Buffer
	if err := printPreflightChecks(&out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := out.String()
	for _, want := range []string{
		"PHASE", "NAME", "APPLIES TO", "PASSES WHEN",
		"registry-reachable",
		"sudo-allowed",
		"--preflight-skip-check=<name>",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the listing must contain %q, got:\n%s", want, text)
		}
	}

	// Configuration checks come first, because that is the order they run in.
	configurationAt := strings.Index(text, "configuration")
	nodesAt := strings.Index(text, "nodes")
	if configurationAt < 0 || nodesAt < 0 || configurationAt > nodesAt {
		t.Errorf("configuration checks must be listed before node checks, got:\n%s", text)
	}
}
