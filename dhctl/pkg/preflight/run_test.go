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

package preflightnew

import (
	"errors"
	"strings"
	"testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// RunSuite is how converge, destroy and check ask their one question. Everything about it that
// differs from the bootstrap runner is deliberate — its own title, no cache — and none of it was
// covered.

// nodePassing and nodeFailing are the recorder's checks in the phase these operations run: the
// machines exist already, so the node-access suite is a post-infra one.
func nodePassing(r *recorder, name CheckName) Check {
	return r.check(name, PhasePostInfra, func() (string, error) { return "", nil })
}

func nodeFailing(r *recorder, name CheckName, err error) Check {
	return r.check(name, PhasePostInfra, func() (string, error) { return "", err })
}

func TestRunSuiteRunsUnderItsOwnTitle(t *testing.T) {
	ctx, buf := testContext(t)
	r := newRecorder()

	err := RunSuite(ctx, NewSuite(nodePassing(r, "ssh-credential")), PhasePostInfra,
		"Preflight checks: converge", nil)
	if err != nil {
		t.Fatalf("a passing suite is not an error: %v", err)
	}

	out := buf.String()
	// The title is the whole reason this exists as its own entry point: three operations share
	// the suite and the reader has to know which one is asking.
	if !strings.Contains(out, "Preflight checks: converge") {
		t.Errorf("want the given title in the report, got:\n%s", out)
	}
	if got := r.count("ssh-credential"); got != 1 {
		t.Errorf("want the check run once, got %d", got)
	}
}

func TestRunSuiteReportsTheFailure(t *testing.T) {
	ctx, _ := testContext(t)
	cause := errors.New("ssh: unable to authenticate")

	err := RunSuite(ctx, NewSuite(nodeFailing(newRecorder(), "ssh-credential", cause)),
		PhasePostInfra, "Preflight checks: destroy", nil)

	if err == nil {
		t.Fatal("a failing check has to reach the caller: the operation must not start")
	}
	if !errors.Is(err, cause) {
		t.Errorf("want the cause reachable through the report, got %v", err)
	}
}

// TestRunSuiteWithNothingToRun: converge against a cluster reached over a kubeconfig has no node
// to look at, and an empty suite must not print an empty box.
func TestRunSuiteWithNothingToRun(t *testing.T) {
	for _, tt := range []struct {
		name  string
		suite Suite
	}{
		{name: "no suite at all", suite: nil},
		{name: "a suite with no checks", suite: NewSuite()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, buf := testContext(t)

			if err := RunSuite(ctx, tt.suite, PhasePostInfra, "Preflight checks: converge", nil); err != nil {
				t.Fatalf("nothing to run is not a failure: %v", err)
			}
			if out := strings.TrimSpace(buf.String()); out != "" {
				t.Errorf("want nothing printed, got:\n%s", out)
			}
		})
	}
}

// TestRunSuiteHonoursTheSkipFlags: Commander has always sent skip names to these operations, and
// until they had checks at all there was nothing for the names to turn off.
func TestRunSuiteHonoursTheSkipFlags(t *testing.T) {
	t.Run("one check by name", func(t *testing.T) {
		ctx, buf := testContext(t)
		r := newRecorder()

		opts := &options.PreflightOptions{SkipChecks: []string{"sudo-allowed"}}
		err := RunSuite(ctx, NewSuite(
			nodePassing(r, "ssh-credential"),
			nodeFailing(r, "sudo-allowed", errors.New("sudo: a password is required")),
		), PhasePostInfra, "Preflight checks: converge", opts)

		if err != nil {
			t.Fatalf("the only failing check was skipped: %v", err)
		}
		if got := r.count("sudo-allowed"); got != 0 {
			t.Errorf("a skipped check must not run, ran %d times", got)
		}
		if out := buf.String(); !strings.Contains(out, "– sudo-allowed") {
			t.Errorf("want the skipped check on its own line, got:\n%s", out)
		}
	})

	t.Run("every check at once", func(t *testing.T) {
		ctx, buf := testContext(t)
		r := newRecorder()

		opts := &options.PreflightOptions{SkipAll: true}
		err := RunSuite(ctx, NewSuite(nodeFailing(r, "ssh-credential", errors.New("no"))),
			PhasePostInfra, "Preflight checks: converge", opts)

		if err != nil {
			t.Fatalf("every check was skipped: %v", err)
		}
		if got := r.count("ssh-credential"); got != 0 {
			t.Errorf("skip-all must run nothing, ran %d times", got)
		}
		if out := buf.String(); !strings.Contains(out, "skipped") {
			t.Errorf("want the skip said out loud, got:\n%s", out)
		}
	})
}
