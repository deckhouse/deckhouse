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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/conversion"
	"github.com/deckhouse/deckhouse/testing/library"
	"github.com/deckhouse/deckhouse/testing/library/values_store"
)

type Converter struct {
	moduleName string
	modulePath string
	values     *values_store.ValuesStore

	FinalValues  *values_store.ValuesStore
	FinalVersion int

	Error error
}

func (c *Converter) ValuesGet(path string) library.KubeResult {
	return c.values.Get(path)
}

func (c *Converter) ValuesSet(path string, value interface{}) {
	c.values.SetByPath(path, value)
}

func (c *Converter) ValuesSetFromYaml(path, value string) {
	c.values.SetByPathFromYAML(path, []byte(value))
}

func (c *Converter) Convert(fromVersion int) {
	chain := conversion.Registry().Chain(c.moduleName)
	Expect(chain).ShouldNot(BeNil(), "Conversion for module %s should be registered", c.moduleName, fromVersion)

	conv := chain.Conversion(fromVersion)
	Expect(conv).ShouldNot(BeNil(), "Conversion for module %s and version %s should be registered", c.moduleName, fromVersion)

	convValues, convError := conv.Convert(conversion.ModuleSettingsFromBytes(c.values.JSONRepr))

	c.FinalValues = values_store.NewStoreFromRawJSON(convValues.Bytes())
	c.FinalVersion = conv.Target
	c.Error = convError
}

func (c *Converter) ConvertToLatest(fromVersion int) {
	modChain := conversion.Registry().Chain(c.moduleName)

	Expect(modChain).ShouldNot(BeNil(), "Module %s should have registered conversions", c.moduleName)

	hasConversion := modChain.IsValidVersion(fromVersion)
	Expect(hasConversion).Should(BeTrue(), "%s version is unknown for module %s: no conversion registered, not the latest one", fromVersion, c.moduleName)

	convVer, convValues, convError := modChain.ConvertToLatest(fromVersion, c.values.Values)

	convValuesJSON, err := json.Marshal(convValues)
	if err != nil {
		c.Error = err
		return
	}

	c.FinalValues = values_store.NewStoreFromRawJSON(convValuesJSON)
	c.FinalVersion = convVer
	c.Error = convError
}

func SetupConverter(values string) *Converter {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	modulePath := filepath.Dir(wd)

	moduleName, err := library.GetModuleNameByPath(modulePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	initialValues, err := library.InitValues(modulePath, []byte(values))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	initialValuesJSON, err := json.Marshal(initialValues)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	converter := &Converter{
		moduleName: moduleName,
		modulePath: modulePath,
	}

	BeforeEach(func() {
		converter.values = values_store.NewStoreFromRawJSON(initialValuesJSON)
	})

	return converter
}
