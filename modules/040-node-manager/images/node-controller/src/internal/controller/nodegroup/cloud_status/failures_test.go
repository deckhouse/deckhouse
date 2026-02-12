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

package cloud_status

import (
	"testing"
	"time"

	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

func TestConvertMachineFailures(t *testing.T) {
	in := []ngcommon.MachineFailure{{
		MachineName: "m1",
		ProviderID:  "pid1",
		OwnerRef:    "owner1",
		Message:     "err",
		Time:        time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
	}}

	out := ConvertMachineFailures(in)
	if len(out) != 1 {
		t.Fatalf("unexpected length: %d", len(out))
	}
	if out[0].Name != "m1" || out[0].ProviderID != "pid1" || out[0].OwnerRef != "owner1" {
		t.Fatalf("unexpected machine failure conversion: %#v", out[0])
	}
	if out[0].LastOperation == nil || out[0].LastOperation.Description != "err" {
		t.Fatalf("expected lastOperation description, got %#v", out[0].LastOperation)
	}
}
