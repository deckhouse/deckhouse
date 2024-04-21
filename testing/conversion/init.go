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
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	d8config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/conversion"
	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/module-manager/test/mock"
	"github.com/deckhouse/deckhouse/testing/library"
)

type ConversionTester struct {
	moduleName string
	modulePath string
}

type ConvTestResult struct {
	Error       error
	Version     int
	Settings    *conversion.Settings
	SettingsMap map[string]interface{}
}

func SetupConversionTester() *ConversionTester {
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

	BeforeEach(func() {
		module, err := mock.NewModule(moduleName, modulePath, mock.EnabledByScript)
		Expect(err).ShouldNot(HaveOccurred(), "should load openapi schemas for module '%s' from '%s'", moduleName, modulePath)

		// Mock module manager with one enabled module.
		mm := mock.NewModuleManager(module)

		d8config.InitService(mm)
	})

	return &ConversionTester{
		moduleName: moduleName,
		modulePath: modulePath,
	}
}

func (c *ConversionTester) TestConversionToNextVersion(fromVersion int, targetVersion int) func(input string, expect string) {
	return func(input string, expect string) {
		res := c.ConvertToNext(fromVersion, input)
		Expect(res.Error).ShouldNot(HaveOccurred())

		expectSettings, err := conversion.SettingsFromYAML(expect)
		Expect(err).ShouldNot(HaveOccurred(), "should convert expected to Settings")

		expectMap, err := expectSettings.Map()
		Expect(err).ShouldNot(HaveOccurred(), "should convert expected Settings to map")

		// A guard for BeComparableTo: it is an error for actual and expected to be nil.
		if len(res.SettingsMap) == 0 && len(expectMap) == 0 {
			return
		}

		Expect(res.Version).Should(Equal(targetVersion), "should convert to next version")
		Expect(res.SettingsMap).To(BeComparableTo(expectMap), "expected settings should not differ from the conversion result")
	}
}

func (c *ConversionTester) TestConversionToValidLatest(fromVersion int) func(input string) {
	return func(input string) {
		res := c.ConvertToLatest(fromVersion, input)
		Expect(res.Error).ShouldNot(HaveOccurred())

		if len(res.SettingsMap) == 0 {
			return
		}

		latest := conversion.Registry().Chain(c.moduleName).LatestVersion()
		Expect(res.Version).Should(Equal(latest), "should convert to latest")
	}
}

func (c *ConversionTester) ConvertToNext(fromVersion int, input string) ConvTestResult {
	res := ConvTestResult{}

	inSettings, err := conversion.SettingsFromYAML(input)
	if err != nil {
		res.Error = fmt.Errorf("input YAML to Settings: %v", err)
		return res
	}

	chain := conversion.Registry().Chain(c.moduleName)
	Expect(chain).ShouldNot(BeNil(), "Conversion for module %s should be registered", c.moduleName, fromVersion)

	conv := chain.Conversion(fromVersion)
	Expect(conv).ShouldNot(BeNil(), "Conversion for module %s and version %s should be registered", c.moduleName, fromVersion)

	resultSettings, err := conv.Convert(inSettings)

	if err != nil {
		res.Error = fmt.Errorf("convert input Settings: %v", err)
		return res
	}
	res.Settings = resultSettings
	res.Version = conv.Target

	res.SettingsMap, err = resultSettings.Map()
	if err != nil {
		res.Error = fmt.Errorf("converted Settings to map: %v", err)
		return res
	}

	return res
}

func (c *ConversionTester) ConvertToLatest(fromVersion int, input string) ConvTestResult {
	res := ConvTestResult{}

	inSettings, err := conversion.SettingsFromYAML(input)
	if err != nil {
		res.Error = fmt.Errorf("input YAML to Settings: %v", err)
		return res
	}

	specSettingsMap, err := inSettings.Map()
	if err != nil {
		res.Error = fmt.Errorf("input Settings to map: %v", err)
		return res
	}

	cfg := &v1alpha1.ModuleConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ModuleConfig",
			APIVersion: "deckhouse.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: c.moduleName,
		},
		Spec: v1alpha1.ModuleConfigSpec{
			Version:  fromVersion,
			Settings: specSettingsMap,
		},
	}

	validationRes := d8config.Service().ConfigValidator().Validate(cfg)
	if validationRes.HasError() {
		res.Error = fmt.Errorf("validate input Settings: %v", validationRes.Error)
		return res
	}

	res.Settings, err = conversion.SettingsFromMap(validationRes.Settings)
	if err != nil {
		res.Error = fmt.Errorf("converted result map to Settings: %v", err)
		return res
	}

	res.SettingsMap, err = res.Settings.Map()
	if err != nil {
		res.Error = fmt.Errorf("result Settings to map: %v", err)
		return res
	}

	res.Version = validationRes.Version
	return res
}
