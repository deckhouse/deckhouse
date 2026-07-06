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

package telemetry

import "testing"

func TestCloudSpanAttributes(t *testing.T) {
	cloud := CloudSpanAttributes("Cloud", "yandex", "WithoutNAT", "e2e-abc", "uuid-1")
	if len(cloud) != 5 {
		t.Fatalf("cloud cluster: want 5 attrs, got %d", len(cloud))
	}

	// Static cluster: no provider/layout, empty uuid -> only type + prefix.
	static := CloudSpanAttributes("Static", "", "", "e2e-static", "")
	if len(static) != 2 {
		t.Fatalf("static cluster: want 2 attrs (type, prefix), got %d", len(static))
	}
	if static[0].Key != "deckhouse.cluster.type" || static[1].Key != "deckhouse.cluster.prefix" {
		t.Fatalf("static cluster: unexpected keys %v / %v", static[0].Key, static[1].Key)
	}

	if got := CloudSpanAttributes("", "", "", "", ""); len(got) != 0 {
		t.Fatalf("all empty: want 0 attrs, got %d", len(got))
	}
}

func TestCommanderSpanAttributes(t *testing.T) {
	withUUID := CommanderSpanAttributes(true, "uuid-1")
	if len(withUUID) != 2 {
		t.Fatalf("with uuid: want 2 attrs, got %d", len(withUUID))
	}
	if withUUID[0].Key != "dhctl.commander_mode" || !withUUID[0].Value.AsBool() {
		t.Fatalf("with uuid: commander_mode not set true")
	}

	// Empty uuid is skipped but mode is always present.
	noUUID := CommanderSpanAttributes(false, "")
	if len(noUUID) != 1 {
		t.Fatalf("no uuid: want 1 attr (mode only), got %d", len(noUUID))
	}
}
