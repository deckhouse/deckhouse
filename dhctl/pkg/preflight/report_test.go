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
	"context"
	"errors"
	"strings"
	"testing"
)

// TestFailureRendersEveryFieldItWasGiven is a golden test on the block a reader acts on. The
// single format it replaced — `preflight check %q failed.\nreason: %w` — named neither the field
// nor the host nor the fix nor the flag, and put a raw json/url/x509 error under `reason:`.
func TestFailureRendersEveryFieldItWasGiven(t *testing.T) {
	ctx, _ := testContext(t)

	check := Check{
		Name:        "cidr-intersection",
		Description: "cluster CIDRs do not intersect",
		Phase:       PhasePreInfra,
		Retry:       NoRetry,
		Run: func(context.Context) (string, error) {
			return "", &Failure{
				Checked:  "ClusterConfiguration.podSubnetCIDR, ClusterConfiguration.serviceSubnetCIDR",
				Observed: "podSubnetCIDR 10.111.0.0/16 overlaps serviceSubnetCIDR 10.111.128.0/17",
				Expected: "disjoint ranges",
				Fix:      "change ClusterConfiguration.podSubnetCIDR or serviceSubnetCIDR",
			}
		},
	}

	err := New(NewSuite(check)).Run(ctx, PhasePreInfra)
	if err == nil {
		t.Fatal("want an error")
	}

	want := `1 preflight check failed (0 passed, 1 failed):

[1] cidr-intersection — cluster CIDRs do not intersect
checked: ClusterConfiguration.podSubnetCIDR, ClusterConfiguration.serviceSubnetCIDR
observed: podSubnetCIDR 10.111.0.0/16 overlaps serviceSubnetCIDR 10.111.128.0/17
expected: disjoint ranges
fix: change ClusterConfiguration.podSubnetCIDR or serviceSubnetCIDR
skip: --preflight-skip-check=cidr-intersection
docs: ` + DocsURL + `

Re-run the same command after fixing, or add the skip flags above to proceed anyway.`

	if err.Error() != want {
		t.Errorf("report mismatch\n--- got ---\n%s\n--- want ---\n%s", err, want)
	}
}

// TestBareErrorStillGetsSkipAndDocs: a check that has not been migrated to *Failure yet must
// still tell the reader the flag that turns it off and the page that describes it.
func TestBareErrorStillGetsSkipAndDocs(t *testing.T) {
	ctx, _ := testContext(t)

	check := Check{
		Name:        "python-modules",
		Description: "python and required modules are installed",
		Phase:       PhasePreInfra,
		Retry:       NoRetry,
		Run: func(context.Context) (string, error) {
			return "", errors.New("python3 is not found on the node")
		},
	}

	err := New(NewSuite(check)).Run(ctx, PhasePreInfra)
	if err == nil {
		t.Fatal("want an error")
	}

	want := `1 preflight check failed (0 passed, 1 failed):

[1] python-modules — python and required modules are installed
reason: python3 is not found on the node
skip: --preflight-skip-check=python-modules
docs: ` + DocsURL + `

Re-run the same command after fixing, or add the skip flags above to proceed anyway.`

	if err.Error() != want {
		t.Errorf("report mismatch\n--- got ---\n%s\n--- want ---\n%s", err, want)
	}
}

// TestUnskippableCheckSaysSoInsteadOfPrintingAFlag: offering a flag the validator will refuse is
// worse than saying there is none.
func TestUnskippableCheckSaysSoInsteadOfPrintingAFlag(t *testing.T) {
	ctx, _ := testContext(t)

	check := Check{
		Name:            "immutable-registry-mode",
		Description:     "registry runs in Unmanaged mode",
		Phase:           PhasePreInfra,
		Retry:           NoRetry,
		CannotBeSkipped: true,
		Run: func(context.Context) (string, error) {
			return "", errors.New("registry mode is Managed")
		},
	}

	err := New(NewSuite(check)).Run(ctx, PhasePreInfra)
	if err == nil {
		t.Fatal("want an error")
	}
	if want := "skip: this check cannot be skipped"; !strings.Contains(err.Error(), want) {
		t.Errorf("want %q in:\n%s", want, err)
	}
}

// TestFailureKeepsTheCauseReachable: errors.As through a *Failure is what lets a caller still
// single out an x509 or a transport error underneath the rendered block.
func TestFailureKeepsTheCauseReachable(t *testing.T) {
	cause := errors.New("certificate signed by unknown authority")
	failure := &Failure{Observed: "TLS verification failed", Err: cause}

	if !errors.Is(failure, cause) {
		t.Error("the cause must stay reachable through the Failure")
	}
	if failure.Error() != "TLS verification failed" {
		t.Errorf("Error() should be the observation, got %q", failure.Error())
	}
}

// TestMultiLineFixIsIndentedUnderItsLabel keeps a two-way fix readable as one field.
func TestMultiLineFixIsIndentedUnderItsLabel(t *testing.T) {
	ctx, _ := testContext(t)

	check := Check{
		Name:        "sudo-allowed",
		Description: "sudo is installed and allowed for user",
		Phase:       PhasePreInfra,
		Retry:       NoRetry,
		Run: func(context.Context) (string, error) {
			return "", &Failure{
				Observed: "sudo: a password is required",
				Fix:      "on the host: echo '...' | sudo tee /etc/sudoers.d/90-dhctl\nor pass the password with --ask-become-pass",
			}
		},
	}

	err := New(NewSuite(check)).Run(ctx, PhasePreInfra)
	if err == nil {
		t.Fatal("want an error")
	}
	if want := "\n     or pass the password with --ask-become-pass\n"; !strings.Contains(err.Error(), want) {
		t.Errorf("continuation line must line up under the label, got:\n%s", err)
	}
}
