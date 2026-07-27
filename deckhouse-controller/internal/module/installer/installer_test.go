/*
Copyright 2025 Flant JSC

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

package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsEmbeddedPresent(t *testing.T) {
	embedded := t.TempDir()

	// Embedded modules are shipped with a weight prefix in their directory name.
	for _, dir := range []string{"040-node-manager", "ingress-nginx", "values-default.yaml"} {
		if err := os.MkdirAll(filepath.Join(embedded, dir), 0755); err != nil {
			t.Fatalf("prepare embedded dir: %v", err)
		}
	}

	cases := []struct {
		name     string
		embedded string
		module   string
		want     bool
	}{
		{name: "prefixed embedded module", embedded: embedded, module: "node-manager", want: true},
		{name: "unprefixed embedded module", embedded: embedded, module: "ingress-nginx", want: true},
		{name: "bare name does not match prefixed dir literally", embedded: embedded, module: "040-node-manager", want: false},
		{name: "absent module", embedded: embedded, module: "cilium", want: false},
		{name: "empty embedded dir setting", embedded: "", module: "node-manager", want: false},
		{name: "missing embedded dir", embedded: filepath.Join(embedded, "does-not-exist"), module: "node-manager", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			i := &Installer{embedded: tc.embedded}
			if got := i.IsEmbeddedPresent(tc.module); got != tc.want {
				t.Errorf("IsEmbeddedPresent(%q) = %v, want %v", tc.module, got, tc.want)
			}
		})
	}
}
