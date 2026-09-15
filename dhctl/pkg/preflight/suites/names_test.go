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

package suites

import (
	"regexp"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks"
)

// TestGeneratedListMatchesSuites is the guard against the drift that put cloud-prefix and
// registry-auth into --help, into --preflight-skip-check validation and into the expansion of
// --preflight-skip-all-checks: both were CheckName constants the generator collected, and
// neither was wired into a suite, so dhctl advertised two checks that could never run.
//
// The generator reads constants out of the source; this reads the suites as they are actually
// built. A name that appears on one side and not the other is a bug in whichever side is newer.
func TestGeneratedListMatchesSuites(t *testing.T) {
	generated := options.GeneratedChecks()
	wired := namesOfEverySuite(t)

	inGeneratedOnly := difference(generated, wired)
	inSuitesOnly := difference(wired, generated)

	if len(inGeneratedOnly) > 0 {
		t.Errorf("names in the generated list but in no suite: %v\n"+
			"either wire the check into a suite, or delete it and re-run `go generate ./hack/gen_preflight_checks`",
			inGeneratedOnly)
	}
	if len(inSuitesOnly) > 0 {
		t.Errorf("names wired into a suite but missing from the generated list: %v\n"+
			"run `go generate ./hack/gen_preflight_checks` — until then --preflight-skip-check refuses these names",
			inSuitesOnly)
	}
}

// namesOfEverySuite collects the names of every check any suite wires.
func namesOfEverySuite(t *testing.T) []string {
	t.Helper()

	seen := map[string]struct{}{}
	for _, check := range everyCheck(t) {
		seen[check.Name.String()] = struct{}{}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// everyCheck builds every suite with zero dependencies and returns the checks they wire. The
// constructors only assemble Check values — the bodies that would dereference the dependencies
// are not called here — so the nil deps are enough to enumerate and inspect them.
func everyCheck(t *testing.T) []preflight.Check {
	t.Helper()

	suites := []preflight.Suite{
		NewGlobalSuite(GlobalDeps{}),
		NewCloudSuite(CloudDeps{}),
		NewPostCloudSuite(PostCloudDeps{}),
		NewImmutableSuite(ImmutableDeps{}),
		NewImmutableStaticSuite(nil),
	}

	suites = append(suites, NewStaticSuite(StaticDeps{}), NewNodeAccessSuite(NodeAccessDeps{}))

	var checks []preflight.Check
	for _, suite := range suites {
		checks = append(checks, suite.Checks()...)
	}
	return checks
}

func difference(a, b []string) []string {
	inB := make(map[string]struct{}, len(b))
	for _, name := range b {
		inB[name] = struct{}{}
	}
	var out []string
	for _, name := range a {
		if _, ok := inB[name]; !ok {
			out = append(out, name)
		}
	}
	return out
}

// descriptionStyle is the shape a Description has to have to read under both a ✓ and a ✗: it is
// an assertion about the cluster, not an imperative and not a sentence. Lower-case first letter
// (the record line puts it after the check name), no trailing period.
var descriptionStyle = regexp.MustCompile(`^[a-z].*[^.]$`)

// TestDescriptionsAreAssertions guards the wording the whole report is built on. The suite used
// to mix imperatives ("validate cluster prefix…", "resolve the localhost domain") with
// assertions, and one description ended in a period while none of the others did — which shows
// up as ragged output the moment the descriptions are printed next to each other.
func TestDescriptionsAreAssertions(t *testing.T) {
	for _, check := range everyCheck(t) {
		if !descriptionStyle.MatchString(check.Description) {
			t.Errorf("%s: description %q must start lower-case and end without a period", check.Name, check.Description)
		}
		if len(check.Description) > 110 {
			t.Errorf("%s: description is %d characters; it shares a line with the check name and the result",
				check.Name, len(check.Description))
		}
	}
}

// TestEveryCheckDeclaresARetryPolicy: a check that leaves Retry unset gets one attempt by
// accident rather than by decision, which is how python-modules and the immutable checks ended
// up with a policy nobody had chosen.
func TestEveryCheckDeclaresARetryPolicy(t *testing.T) {
	for _, check := range everyCheck(t) {
		if check.Retry.Attempts <= 0 {
			t.Errorf("%s declares no retry policy; set preflight.NoRetry or preflight.NetworkRetry explicitly", check.Name)
		}
	}
}

// TestUnskippableChecksMatchTheSuites keeps the list --preflight-skip-check refuses in step with
// the checks that actually say they cannot be skipped. The two live apart — the flag layer cannot
// import the suites without a cycle — and a guard that is unskippable in one place and skippable
// in the other is worse than having no guard: --preflight-skip-all-checks would quietly turn it
// off while the flag pretends it cannot.
func TestUnskippableChecksMatchTheSuites(t *testing.T) {
	declared := map[string]struct{}{}
	for _, check := range everyCheck(t) {
		if check.CannotBeSkipped {
			declared[check.Name.String()] = struct{}{}
		}
	}

	refused := map[string]struct{}{}
	for _, name := range options.GeneratedChecks() {
		o := &options.PreflightOptions{SkipChecks: []string{name}}
		if err := o.Normalize(); err != nil {
			refused[name] = struct{}{}
		}
	}

	for name := range declared {
		if _, ok := refused[name]; !ok {
			t.Errorf("check %q sets CannotBeSkipped but --preflight-skip-check=%s is accepted", name, name)
		}
	}
	for name := range refused {
		if _, ok := declared[name]; !ok {
			t.Errorf("--preflight-skip-check=%s is refused but the check does not set CannotBeSkipped", name)
		}
	}

	// And the flag that skips everything must not skip these either.
	skipAll := &options.PreflightOptions{SkipAll: true}
	for _, name := range skipAll.DisabledChecks() {
		if _, ok := declared[name]; ok {
			t.Errorf("--preflight-skip-all-checks disables %q, which cannot be skipped", name)
		}
	}
}

// TestNodeChecksAreNeverCached pins the decision that node results are not remembered at all.
//
// The cache key is built from the configuration files, and nothing about a machine is a function
// of those: the machine behind an address can be replaced between two runs, a user can be added,
// a port can be taken. A remembered ✓ would then be about a machine that no longer exists — which
// is what --drop-cache, the only invalidation there was, existed to work around.
func TestNodeChecksAreNeverCached(t *testing.T) {
	for _, check := range everyCheck(t) {
		if !check.Cacheable {
			continue
		}
		if check.Phase != preflight.PhasePreInfra {
			t.Errorf("check %q is cacheable and runs in the %s phase; only checks decided by the configuration may be remembered",
				check.Name, check.Phase)
		}
	}
}

// TestRegistryIsFourChecksNotOne pins the shape chosen for the registry: reachable, then
// authenticated to, then asked for the image, then asked the same from the node. One check
// answering all of that is how six unrelated causes came to share the sentence "authentication
// failed"; four checks cost three extra round trips and name what is actually wrong.
func TestRegistryIsFourChecksNotOne(t *testing.T) {
	byName := map[preflight.CheckName]preflight.Check{}
	for _, check := range everyCheck(t) {
		byName[check.Name] = check
	}

	for _, name := range []preflight.CheckName{
		"registry-reachable",
		"registry-credentials",
		"deckhouse-image-available",
		"registry-access-from-master",
	} {
		if _, ok := byName[name]; !ok {
			t.Errorf("check %q left the suites; the registry chain is meant to stay split", name)
		}
	}

	// And each step declares the one before it, so an unreachable registry is one failure.
	if deps := byName["registry-credentials"].DependsOn; len(deps) == 0 {
		t.Error("registry-credentials must depend on registry-reachable, or an unreachable registry fails twice")
	}
	if deps := byName["deckhouse-image-available"].DependsOn; len(deps) == 0 {
		t.Error("deckhouse-image-available must depend on the registry being reachable and authenticated to")
	}
}

// TestOnlyTheSSHConnectionStopsThePhase pins which checks are allowed to end a phase early.
//
// It is the connection every node check is asked over, and nothing else: a check that merely
// makes another one pointless uses DependsOn, which reports that one blocked and carries on. The
// distinction matters because a phase that stops has no record for the checks it never reached,
// and an operator reading the output cannot tell they exist.
func TestOnlyTheSSHConnectionStopsThePhase(t *testing.T) {
	allowed := map[string]struct{}{
		"ssh-connectivity": {},
		"ssh-credential":   {},
	}

	for _, check := range everyCheck(t) {
		if !check.StopsPhaseOnFailure {
			continue
		}
		if _, ok := allowed[check.Name.String()]; !ok {
			t.Errorf("check %q ends the phase on failure; only the SSH connection is meant to, "+
				"everything else uses DependsOn", check.Name)
		}
	}
}

// TestCloudMasterCredentialIsCheckedFirst is the answer to a cloud bootstrap that reported
// "cannot open tunnel … check that sshd has AllowTcpForwarding yes" when the truth was a wrong
// --ssh-user.
//
// The cloud suite had no credential check at all, so the first check to touch SSH was whichever
// one ran first, and each reported the failure in its own terms: the cloud API check saw a
// port-forward that would not open and blamed sshd's forwarding settings — on a machine nobody
// had logged in to. Asking the question once, before anything tunnels, is the fix; stopping the
// phase is what keeps the rest from repeating the guess.
func TestCloudMasterCredentialIsCheckedFirst(t *testing.T) {
	suite := NewPostCloudSuite(PostCloudDeps{})

	var credentialAt = -1
	var firstOverSSHAt = -1
	for i, check := range suite.Checks() {
		switch check.Name {
		case checks.SSHCredentialCheckName:
			credentialAt = i
			if !check.StopsPhaseOnFailure {
				t.Error("the credential check has to end the phase: everything after it is asked over that connection")
			}
		case checks.CloudAPICheckName, checks.RegistryFromMasterCheckName:
			if firstOverSSHAt == -1 || i < firstOverSSHAt {
				firstOverSSHAt = i
			}
		}
	}

	if credentialAt == -1 {
		t.Fatal("the cloud suite must check that dhctl can log in to the master it just created")
	}
	if firstOverSSHAt != -1 && credentialAt > firstOverSSHAt {
		t.Errorf("the credential is checked at position %d, after a check that tunnels through it at %d",
			credentialAt, firstOverSSHAt)
	}
}

// TestCloudAPIDependsOnTheCredential: the cloud API check used to wait for the master to answer
// before doing anything of its own. That wait now belongs to ssh-credential, which runs first — so
// the dependency is declared rather than left implicit, and a reader of the report sees the cloud
// API check blocked by the credential rather than failing on its own account.
func TestCloudAPIDependsOnTheCredential(t *testing.T) {
	for _, check := range NewPostCloudSuite(PostCloudDeps{}).Checks() {
		if check.Name != checks.CloudAPICheckName {
			continue
		}
		assert.Contains(t, check.DependsOn, checks.SSHCredentialCheckName)
		return
	}
	t.Fatalf("%s is not in the cloud suite", checks.CloudAPICheckName)
}
