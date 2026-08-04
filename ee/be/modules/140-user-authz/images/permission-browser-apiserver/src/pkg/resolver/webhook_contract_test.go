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

	assert.Equal(t, pythonSet(t, source, "SUPERADMIN_ROLES"), superadminRoles, "SUPERADMIN_ROLES")
	assert.Equal(t, pythonSet(t, source, "CLUSTER_ADMIN_ROLES"), clusterAdminRoles, "CLUSTER_ADMIN_ROLES")
	// Equality, in both directions. The subset check this replaces would have let a group added on
	// the python side pass unnoticed, and the report would go on claiming a restriction the webhook
	// had stopped applying.
	assert.Equal(t, pythonSet(t, source, "BYPASS_GROUPS"), bypassGroups, "BYPASS_GROUPS")
}

// webhookTestPath is the test that ships beside the webhook. The two travel together, so their
// absence tells the checkout apart from the drift.
const webhookTestPath = "../../../../../../../../../modules/140-user-authz/webhooks/validating/system_resources_test.py"

func readWebhookSource(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(webhookSourcePath))
	if err == nil {
		return string(data)
	}

	// A missing file is the drift this test exists to catch -- renaming or deleting the webhook
	// would otherwise turn the test green -- so it fails, unless the webhook is not part of this
	// checkout at all. That case is real: the image is built from this subtree alone, and the two
	// halves of the model live in different pull requests until they merge. It is told apart by the
	// test that ships next to the webhook: gone together, the whole thing is absent; gone alone, the
	// source moved and the contract can no longer be checked.
	if _, sibling := os.Stat(filepath.Clean(webhookTestPath)); sibling != nil {
		t.Skipf("the user-authz webhooks are not part of this checkout (%v)", err)
	}

	t.Fatalf("the webhook this package models is gone from %s: %v", webhookSourcePath, err)

	return ""
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
