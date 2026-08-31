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

package validate

import "testing"

func TestOutputZeroValueIsUsable(t *testing.T) {
	var output Output

	if output.HasErrors() {
		t.Error("HasErrors() = true on a zero Output")
	}
	if output.HasWarnings() {
		t.Error("HasWarnings() = true on a zero Output")
	}
	if got := output.Errors().String(); got != "" {
		t.Errorf("Errors().String() = %q, want empty", got)
	}
}

// The host prints this text verbatim and plugins compare whole strings, so the
// order is part of the contract.
func TestOutputOrdersViolationsByCodeThenPath(t *testing.T) {
	var output Output
	output.AddError("z", "b_code", nil, "second")
	output.AddError("a", "a_code", nil, "first")
	output.AddError("b", "a_code", nil, "middle")

	want := "a: first\nb: middle\nz: second"
	if got := output.Errors().String(); got != want {
		t.Fatalf("Errors().String() =\n%s\nwant\n%s", got, want)
	}
}

func TestOutputErrorOmitsEmptyPath(t *testing.T) {
	var output Output
	output.AddError("", "internal", nil, "state is not initialized")

	if got := output.Errors().String(); got != "state is not initialized" {
		t.Fatalf("Errors().String() = %q", got)
	}
}

// Warnings never turn a valid configuration into an invalid one.
func TestOutputWarningsAreNotBlocking(t *testing.T) {
	var output Output
	output.AddWarning("NodeGroup/worker", "replicas_zero", nil, "replicas is 0")

	if output.HasErrors() {
		t.Error("HasErrors() = true with only warnings")
	}
	if !output.HasWarnings() {
		t.Error("HasWarnings() = false with a warning")
	}
	if got := len(output.Warnings()); got != 1 {
		t.Errorf("Warnings() = %d, want 1", got)
	}
}

func TestOutputRoundTripKeepsTextAndKinds(t *testing.T) {
	var output Output
	output.AddError("NodeGroup/master", "node_group_required", nil, `NodeGroup "master" is required`)
	output.AddError("Secret/d8-credentials.data.secret", "invalid_kubeconfig", "masked", "invalid kubeconfig")
	output.AddWarning("", "layout_empty", nil, "layout is not set")

	got := FromPBResponse(ToPBResponse(output))

	if got.Errors().String() != output.Errors().String() {
		t.Fatalf("Errors().String() =\n%s\nwant\n%s", got.Errors().String(), output.Errors().String())
	}
	if len(got.Warnings()) != 1 {
		t.Fatalf("Warnings() = %d, want 1", len(got.Warnings()))
	}
	if value := got.Errors()[0].Value; value != "masked" {
		t.Errorf("Value = %v, want %q", value, "masked")
	}
}

// A non-string Value is rendered, not dropped: rules report rejected values of any
// type and the wire form is display-only.
func TestViolationsRenderNonStringValue(t *testing.T) {
	var output Output
	output.AddError("DVPInstanceClass/worker.spec.etcdDisk", "etcd_disk_forbidden", 3, "etcdDisk is not allowed here")

	resp := ToPBResponse(output)

	if got := resp.GetErrors()[0].GetValue(); got != "3" {
		t.Fatalf("Value = %q, want %q", got, "3")
	}
}

func TestOutputOfNilResponseIsEmpty(t *testing.T) {
	output := FromPBResponse(nil)

	if output.HasErrors() || output.HasWarnings() {
		t.Fatalf("Output = %+v, want empty", output)
	}
}
