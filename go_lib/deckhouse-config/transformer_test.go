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

package deckhouse_config

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/conversion"
	d8cfg_v1alpha1 "github.com/deckhouse/deckhouse/go_lib/deckhouse-config/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

func TestTransformer(t *testing.T) {
	g := NewWithT(t)

	conversion.RegisterFunc("global", 1, 2, func(settings *conversion.Settings) error {
		return settings.Set("params.someParam", "newValue")
	})
	conversion.RegisterFunc("module-one", 1, 2, func(settings *conversion.Settings) error {
		return settings.Set("params.someParam", "newValue")
	})
	conversion.RegisterFunc("module-two", 1, 2, func(settings *conversion.Settings) error {
		return settings.DeleteAndClean("params.params.param1")
	})
	conversion.RegisterFunc("module-four", 1, 2, func(settings *conversion.Settings) error {
		return settings.DeleteAndClean("params.params.param1")
	})

	cmData := map[string]string{
		"moduleOne": `{}`,
		"moduleTwo": `
params:
  params:
    param1: value1
`,
		"moduleTwoEnabled":   "false",
		"moduleThreeEnabled": "false",
		"moduleFour": `
params:
  params:
    param1: value1
`,
		"moduleFive": `
params:
  params:
    param1: value1
`,
	}

	possibleNames := set.New(
		"global",
		"module-one",
		"module-two",
		"module-three",
		"module-four",
	)

	tr := NewTransformer(possibleNames)

	cfgList, msgs, err := tr.ConfigMapToModuleConfigList(cmData)

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(cfgList).ShouldNot(BeEmpty())

	cfgMap := make(map[string]*d8cfg_v1alpha1.ModuleConfig)
	for _, cfg := range cfgList {
		cfgMap[cfg.Name] = cfg
	}

	// global — no values, has conversion, should not have enabled flag.
	name := "global"
	ensureReport(t, msgs, name)
	ensureConfigIsPresent(t, cfgMap, name)
	g.Expect(cfgMap[name]).Should(And(
		HaveField("Spec.Enabled", BeNil()),
		HaveField("Spec.Version", 2),
		HaveField("Spec.Settings", Not(BeEmpty())),
	))

	// module-one — empty values, has conversions, should convert values, should not have enabled field.
	name = "module-one"
	ensureReport(t, msgs, name)
	ensureConfigIsPresent(t, cfgMap, name)
	g.Expect(cfgMap[name]).Should(And(
		HaveField("Spec.Enabled", BeNil()),
		HaveField("Spec.Version", 2),
		HaveField("Spec.Settings", Not(BeEmpty())),
	))

	// module-two — has values, has conversions, should convert to empty values, should have enabled field.
	name = "module-two"
	ensureReport(t, msgs, name)
	ensureConfigIsPresent(t, cfgMap, name)
	g.Expect(cfgMap[name]).Should(And(
		HaveField("Spec.Enabled", Not(BeNil())),
		HaveField("Spec.Version", Equal(0)),
		HaveField("Spec.Settings", BeEmpty()),
	))

	// module-three — no values, no conversions, should have only enabled field.
	name = "module-three"
	ensureReport(t, msgs, name)
	ensureConfigIsPresent(t, cfgMap, name)
	g.Expect(cfgMap[name]).Should(And(
		HaveField("Spec.Enabled", Not(BeNil())),
		HaveField("Spec.Version", Equal(0)),
		HaveField("Spec.Settings", BeEmpty()),
	))

	// module-four — values are converted to empty values, no enabled flag, should ignore empty object.
	name = "module-four"
	ensureReport(t, msgs, name)
	g.Expect(cfgMap).ShouldNot(HaveKey(name), "should not have '%s'", name)

	// module-five — unknown.
	name = "module-five"
	ensureReport(t, msgs, name)
	g.Expect(cfgMap).ShouldNot(HaveKey(name), "should not have '%s'", name)
}

func ensureReport(t *testing.T, msgs []string, name string) {
	g := NewWithT(t)
	g.Expect(msgs).Should(ContainElement(ContainSubstring(name)), "should report about transformation '%s', got:\n%s", name, strings.Join(msgs, "\n"))
}

func ensureConfigIsPresent(t *testing.T, cfgMap map[string]*d8cfg_v1alpha1.ModuleConfig, name string) {
	g := NewWithT(t)
	g.Expect(cfgMap).Should(HaveKey(name), "should have '%s', got objects: %+v", name, cfgMap)
}
