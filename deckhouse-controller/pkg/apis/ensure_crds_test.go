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

package apis

import (
	"testing"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
)

// Run under every build tag set of editions.yaml: the licensing CRDs follow the
// build, everything else is installed regardless.
func TestInstallCRDFile(t *testing.T) {
	if installCRDFile("/deckhouse/deckhouse-controller/crds/doc-ru-module.yaml") {
		t.Error("a translated doc file was installed")
	}
	if !installCRDFile("/deckhouse/deckhouse-controller/crds/module.yaml") {
		t.Error("a regular CRD was skipped")
	}
	for _, name := range licensingCRDs {
		if got := installCRDFile("/deckhouse/deckhouse-controller/crds/" + name); got != app.LicensingEnabled() {
			t.Errorf("%s installed=%v, licensing enabled=%v", name, got, app.LicensingEnabled())
		}
	}
}
