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

package template_tests

import (
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// registry/registry-config-secret.yaml carries the deckhouse.registry settings of the mode in force to
// the previous registry implementation. When it is rendered at all is checked in module_test.go ("The
// legacy registry contour"); these tests check what it carries in each mode.

const registryTestPassword = "dh-test-secret-0123456789abcdef"

var _ = Describe("Module :: deckhouse :: legacy registry configuration ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
	})

	// registryConfig renders the chart with the given deckhouse.registry settings and returns the
	// decoded data of the Secret.
	registryConfig := func(settings string) map[string]string {
		f.ValuesSetFromYaml("deckhouse.registry", settings)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		secret := f.KubernetesResource("Secret", "d8-system", "registry-config")
		Expect(secret.Exists()).To(BeTrue())
		Expect(secret.Field("type").String()).To(Equal("registry/config"))

		data := map[string]string{}
		for key, value := range secret.Field("data").Map() {
			decoded, err := base64.StdEncoding.DecodeString(value.String())
			Expect(err).ShouldNot(HaveOccurred(), key)
			data[key] = string(decoded)
		}
		return data
	}

	It("carries the mode alone in the Unmanaged mode without settings of its own", func() {
		Expect(registryConfig(`{mode: Unmanaged}`)).To(Equal(map[string]string{"mode": "Unmanaged"}))
	})

	It("carries a license as the password of the license-token user", func() {
		Expect(registryConfig(`
mode: Unmanaged
unmanaged:
  imagesRepo: registry.example.com/deckhouse/ee
  scheme: HTTPS
  license: ` + registryTestPassword + `
  checkMode: Default
`)).To(Equal(map[string]string{
			"mode":       "Unmanaged",
			"imagesRepo": "registry.example.com/deckhouse/ee",
			"scheme":     "HTTPS",
			"username":   "license-token",
			"password":   registryTestPassword,
			"checkMode":  "Default",
		}))
	})

	It("carries the account and the CA of the Direct mode", func() {
		Expect(registryConfig(`
mode: Direct
direct:
  imagesRepo: registry.example.com/deckhouse
  scheme: HTTP
  username: registry-user
  password: ` + registryTestPassword + `
  ca: example-ca
  checkMode: Relax
`)).To(Equal(map[string]string{
			"mode":       "Direct",
			"imagesRepo": "registry.example.com/deckhouse",
			"scheme":     "HTTP",
			"username":   "registry-user",
			"password":   registryTestPassword,
			"ca":         "example-ca",
			"checkMode":  "Relax",
		}))
	})

	It("carries no account to a registry that needs none", func() {
		Expect(registryConfig(`
mode: Direct
direct:
  imagesRepo: registry.example.com/deckhouse
  scheme: HTTPS
  checkMode: Default
`)).To(Equal(map[string]string{
			"mode":       "Direct",
			"imagesRepo": "registry.example.com/deckhouse",
			"scheme":     "HTTPS",
			"checkMode":  "Default",
		}))
	})

	It("carries the cache lifetime of the Proxy mode when it is set", func() {
		Expect(registryConfig(`
mode: Proxy
proxy:
  imagesRepo: registry.example.com/deckhouse/ee
  scheme: HTTPS
  license: ` + registryTestPassword + `
  checkMode: Default
  ttl: 168h
`)).To(Equal(map[string]string{
			"mode":       "Proxy",
			"imagesRepo": "registry.example.com/deckhouse/ee",
			"scheme":     "HTTPS",
			"username":   "license-token",
			"password":   registryTestPassword,
			"checkMode":  "Default",
			"ttl":        "168h",
		}))
	})

	It("carries no cache lifetime of the Proxy mode when it is not set", func() {
		Expect(registryConfig(`
mode: Proxy
proxy:
  imagesRepo: registry.example.com/deckhouse/ee
  scheme: HTTPS
  checkMode: Default
`)).To(Equal(map[string]string{
			"mode":       "Proxy",
			"imagesRepo": "registry.example.com/deckhouse/ee",
			"scheme":     "HTTPS",
			"checkMode":  "Default",
		}))
	})

	It("carries the mode alone in the Local mode, which has no settings", func() {
		Expect(registryConfig(`{mode: Local}`)).To(Equal(map[string]string{"mode": "Local"}))
	})
})
