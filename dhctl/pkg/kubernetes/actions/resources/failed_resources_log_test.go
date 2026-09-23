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

package resources

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

func readinessCheckerFor(group, version, kind, namespace, name string) *resourceReadinessChecker {
	obj := unstructured.Unstructured{}
	obj.SetName(name)
	obj.SetNamespace(namespace)

	return &resourceReadinessChecker{
		resource: &template.Resource{
			GVK:    schema.GroupVersionKind{Group: group, Version: version, Kind: kind},
			Object: obj,
		},
	}
}

// TestLogFailedResourcesKeepsTheBoxIntact pins the framing: every line the report emits between
// the ┌ and └ borders must carry the │ box prefix. Logging the body at Warn level used to route it
// past the box, printing the resource list flush against the left margin and tearing the block in
// half.
func TestLogFailedResourcesKeepsTheBoxIntact(t *testing.T) {
	var buf bytes.Buffer
	ctx := dhlog.ToContext(t.Context(), dhlog.NewStreamLogger(&buf))

	logFailedResources(ctx, 30*time.Minute, []Checker{
		readinessCheckerFor("", "v1", "Secret", "d8-commander-agent", "auth-token"),
		readinessCheckerFor("deckhouse.io", "v1", "IngressNginxController", "", "nginx"),
	})

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")

	// The renderer trails a closed block with a separator and a persistent FAILED milestone;
	// the box itself is everything up to and including the └ line.
	closeAt := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "└ ") {
			closeAt = i
			break
		}
	}
	require.NotEqual(t, -1, closeAt, "the block is never closed:\n%s", buf.String())

	require.True(t, strings.HasPrefix(lines[0], "┌ "), "first line opens the box: %q", lines[0])
	require.Contains(t, lines[closeAt], "FAILED", "the block is closed as failed, not as a success")
	require.NotContains(t, lines[closeAt], "seconds", "a report block does not report a duration")

	for _, line := range lines[1:closeAt] {
		require.True(t, strings.HasPrefix(line, "│"), "body line escaped the box: %q\nfull output:\n%s", line, buf.String())
	}

	require.Contains(t, buf.String(), "d8-commander-agent/auth-token")
	require.Contains(t, buf.String(), "IngressNginxController (deckhouse.io/v1):")
	require.Contains(t, buf.String(), "30m0s")
}
