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
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// The deckhouse-discovery Secret hands the module configuration to the controllers that read it
// from the cluster: the module release controller takes the update notifications from
// updateSettings.json, and licensing takes the cluster UUID the license keys are issued for.

var _ = Describe("Module :: deckhouse :: discovery secret ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
	})

	discovery := func() object_store.KubeObject {
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		secret := f.KubernetesResource("Secret", "d8-system", "deckhouse-discovery")
		Expect(secret.Exists()).To(BeTrue())
		return secret
	}

	decoded := func(secret object_store.KubeObject, key string) string {
		value, err := base64.StdEncoding.DecodeString(secret.Field("data." + key).String())
		Expect(err).ShouldNot(HaveOccurred())
		return string(value)
	}

	It("publishes the bundle and the release channel", func() {
		f.ValuesSet("deckhouse.bundle", "Minimal")
		f.ValuesSet("deckhouse.releaseChannel", "EarlyAccess")
		secret := discovery()

		Expect(decoded(secret, "bundle")).To(Equal("Minimal"))
		Expect(decoded(secret, "releaseChannel")).To(Equal("EarlyAccess"))
	})

	It("reports the release channel as Unknown while none is set", func() {
		Expect(decoded(discovery(), "releaseChannel")).To(Equal("Unknown"))
	})

	It("publishes the update settings as JSON when they are set", func() {
		f.ValuesSetFromYaml("deckhouse.update", `{mode: Auto, notification: {webhook: "https://release-notes.example.com/hook", minimalNotificationTime: 6h}}`)

		Expect(decoded(discovery(), `updateSettings\.json`)).
			To(MatchJSON(`{"mode":"Auto","notification":{"webhook":"https://release-notes.example.com/hook","minimalNotificationTime":"6h"}}`))
	})

	It("leaves the update settings out when there are none", func() {
		Expect(discovery().Field(`data.updateSettings\.json`).Exists()).To(BeFalse())
	})

	It("publishes the cluster UUID once it is discovered", func() {
		f.ValuesSet("global.discovery.clusterUUID", "f76f54dc-7ea0-11ec-899e-c70701aef75e")

		Expect(decoded(discovery(), "clusterUUID")).To(Equal("f76f54dc-7ea0-11ec-899e-c70701aef75e"))
	})

	It("leaves the cluster UUID out until it is discovered", func() {
		Expect(discovery().Field("data.clusterUUID").Exists()).To(BeFalse())
	})
})
