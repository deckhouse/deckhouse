/*
Copyright 2022 Flant JSC

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

package conversion

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestConvertConfigValuesToLatest(t *testing.T) {
	g := NewWithT(t)

	const modName = "test-mod"
	RegisterFunc(modName, 1, 2, func(settings *Settings) error {
		return settings.Set("param2", "val2")
	})

	v0Vals := map[string]interface{}{
		"param1": "val1",
	}
	chain := Registry().Chain(modName)
	g.Expect(chain).ShouldNot(BeNil())
	newVer, newVals, err := chain.ConvertToLatest(1, v0Vals)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(newVer).Should(Equal(2))
	g.Expect(newVals).Should(HaveKey("param1"), "should keep old params")
	g.Expect(newVals).Should(HaveKey("param2"), "should add new param")
}
