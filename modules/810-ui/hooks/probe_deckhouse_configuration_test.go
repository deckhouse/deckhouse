/*
Copyright 2023 Flant JSC

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

package hooks

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

var _ = Describe("Modules :: upmeter :: hooks :: probe_deckhouse_configuration ::", func() {
	Context("filterProbeObject ", func() {
		It("correctly fills snapshot", func() {
			name := "qwerty"
			initial := "ZZZZZZ"
			expectedSnapshot := probeObject{Name: name, Inited: initial}
			obj := newUpmeterHookProbeObject(name, initial)

			rawSnapshot, err := filterProbeObject(obj)
			Expect(err).ToNot(HaveOccurred())

			snapshot := rawSnapshot.(probeObject)
			Expect(snapshot).To(Equal(expectedSnapshot))
		})
	})
})

func newUpmeterHookProbeObject(name, initedValue string) *unstructured.Unstructured {
	template := `
apiVersion: deckhouse.io/v1
kind: UpmeterHookProbe
metadata:
  name: %s
spec:
  inited: %s
  mirror: "<>"
`

	manifest := fmt.Sprintf(template, name, initedValue)

	decUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	if _, _, err := decUnstructured.Decode([]byte(manifest), nil, obj); err != nil {
		panic("cannot decode YAML to unstructured.Unstructured")
	}

	return obj
}
