/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// webhookSourcePath is the admission webhook this file models. The report says which restrictions
// apply to a subject, so it has to agree with the code that actually applies them.
const webhookSourcePath = "../../../../../../../../../modules/140-user-authz/webhooks/validating/system_resources.py"

// TestSuperadminSetsMatchTheWebhook is a contract test: it reads the sets out of the webhook source
// and compares them with the ones this package matches on.
//
// The two live in different languages and different modules, and a comment asking the next person to
// keep them in sync is exactly the kind of promise that quietly rots. Drift is not symmetric in cost:
// a set that grew here claims a bypass the webhook does not grant, and a report claiming a
// restriction that is not applied -- or the reverse -- is worse than no report at all.
func TestSuperadminSetsMatchTheWebhook(t *testing.T) {
	source := readWebhookSource(t)

	assert.Equal(t, pythonSet(t, source, "SUPERADMIN_ROLES"), goSet(superadminRoles), "SUPERADMIN_ROLES")
	assert.Equal(t, pythonSet(t, source, "CLUSTER_ADMIN_ROLES"), goSet(clusterAdminRoles), "CLUSTER_ADMIN_ROLES")

	// BYPASS_GROUPS also lists cluster components (system:nodes, the system service accounts). They
	// bypass the webhook as well, but a report is never built for them, so only the administrator
	// groups are mirrored -- hence a subset check rather than equality.
	webhookGroups := pythonSet(t, source, "BYPASS_GROUPS")
	for group := range bypassGroups {
		assert.Contains(t, webhookGroups, group, "group is not bypassed by the webhook")
	}
}

func readWebhookSource(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(webhookSourcePath))
	if err != nil {
		// The webhook lives in another module of the same repository. When only this image is
		// checked out there is nothing to compare against, and failing would report a packaging
		// detail as a contract violation.
		t.Skipf("webhook source is not available (%v)", err)
	}

	return string(data)
}

var pythonStringRE = regexp.MustCompile(`"([^"]+)"`)

// pythonSet extracts the string literals of a `NAME = { ... }` block, ignoring comments.
func pythonSet(t *testing.T, source, name string) map[string]struct{} {
	t.Helper()

	start := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `\s*=\s*\{`).FindStringIndex(source)
	require.NotNil(t, start, "%s is not defined in the webhook source", name)

	end := start[1]
	for depth := 1; depth > 0; end++ {
		require.Less(t, end, len(source), "%s is not closed", name)
		switch source[end] {
		case '{':
			depth++
		case '}':
			depth--
		}
	}

	set := map[string]struct{}{}
	for _, line := range strings.Split(source[start[1]:end-1], "\n") {
		if code := stripComment(line); code != "" {
			for _, match := range pythonStringRE.FindAllStringSubmatch(code, -1) {
				set[match[1]] = struct{}{}
			}
		}
	}

	require.NotEmpty(t, set, "%s parsed as empty", name)

	return set
}

func stripComment(line string) string {
	if i := strings.Index(line, "#"); i >= 0 {
		return line[:i]
	}

	return line
}

func goSet(set map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(set))
	for key := range set {
		out[key] = struct{}{}
	}

	return out
}
