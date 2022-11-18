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
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
)

func TestConversionError(t *testing.T) {
	g := NewWithT(t)
	c := Conversion{
		Source: 1,
		Target: 2,
	}

	vals := SettingsFromString(`{"params":[{"param1":{"name":"value"}}]}`)

	c.Conversion = func(settings *Settings) error {
		return settings.Delete("params.0")
	}
	newVals, err := c.Convert(vals)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(newVals.Get("params.0").Exists()).Should(BeFalse(), "should delete path")

	c.Conversion = func(settings *Settings) error {
		_ = settings.Delete("params.0.param1.name")
		return fmt.Errorf("oops")
	}
	newVals, err = c.Convert(vals)
	g.Expect(err).Should(HaveOccurred(), "should return error")
	g.Expect(vals.Get("params.0.param1.name").Exists()).Should(BeTrue(), "should not modify values on error")
}
