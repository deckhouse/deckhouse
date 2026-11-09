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

package bashible

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWrittenByInstaller(t *testing.T) {
	document := []byte("mode: Direct\nimagesBase: registry.d8-system.svc:5001/system/deckhouse\n")
	marked := map[string]string{InstallerMarkAnnotation: InstallerMark(document)}

	cases := []struct {
		name        string
		annotations map[string]string
		document    []byte
		want        bool
	}{{
		name:        "the document the installer wrote",
		annotations: marked,
		document:    document,
		want:        true,
	}, {
		name:        "no mark, which is every document a v1 writer left",
		annotations: map[string]string{"helm.sh/resource-policy": "keep"},
		document:    document,
	}, {
		name:        "rewritten since the installer marked it",
		annotations: marked,
		document:    []byte("mode: Proxy\nproxyEndpoints:\n- 192.168.199.10:5001\n"),
	}, {
		name:        "a mark with no document",
		annotations: map[string]string{InstallerMarkAnnotation: InstallerMark(nil)},
	}, {
		name:        "a mark that is not a digest of anything",
		annotations: map[string]string{InstallerMarkAnnotation: "true"},
		document:    document,
	}}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, WrittenByInstaller(test.annotations, test.document))
		})
	}
}
