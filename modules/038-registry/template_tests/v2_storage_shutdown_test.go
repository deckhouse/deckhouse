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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// How a replica of the store goes away.
//
// The syncer releases the fill lease on SIGTERM, and until another replica takes it the nodes
// still pull from this one: the layout controller keeps the Storage endpoint where it was while
// the lease is empty. So the registry has to keep serving through that handover, and so does the
// token service it exchanges every client's credentials with, while the syncer must not wait at
// all. Each half of that is one line in the template, and dropping either is silent until an
// air-gapped cluster fails its pulls during an update.
var _ = Describe("Module :: registry :: helm template :: v2 storage shutdown", func() {
	f := SetupHelmConfig(``)

	// The longest a container is given here for its own stop, after its wait: the token service
	// finishing the exchanges in flight, the syncer releasing the lease and closing its listener.
	// The registry itself stops at once.
	const stopAllowance = 10

	// The part of the handover the registry's wait has to cover at the least: a follower tries for
	// the lease every five to eleven seconds (five with jitter), and the layouts are rewritten as
	// soon as it holds it. An agent that has not read the new layout by the time the registry stops
	// reads it on its first failed request to it, so that agent sees one failure and no more.
	const handover = 15

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", v2Enabled)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	containers := func() map[string]gjson.Result {
		sts := f.KubernetesResource("StatefulSet", "d8-system", "registry-storage")
		Expect(sts.Exists()).To(BeTrue())

		out := map[string]gjson.Result{}
		for _, c := range sts.Field("spec.template.spec.containers").Array() {
			out[c.Get("name").String()] = c
		}
		return out
	}

	gracePeriod := func() int64 {
		sts := f.KubernetesResource("StatefulSet", "d8-system", "registry-storage")
		grace := sts.Field("spec.template.spec.terminationGracePeriodSeconds")
		Expect(grace.Exists()).To(BeTrue())
		return grace.Int()
	}

	It("keeps the registry serving until the lease has moved", func() {
		wait := containers()["distribution"].Get("lifecycle.preStop.sleep.seconds")
		Expect(wait.Exists()).To(BeTrue(),
			"without a wait the registry stops with the syncer, while the nodes still pull from it")
		Expect(wait.Int()).To(BeNumerically(">=", handover))
	})

	It("keeps the token service up for at least as long as the registry", func() {
		all := containers()
		registry := all["distribution"].Get("lifecycle.preStop.sleep.seconds")
		auth := all["auth"].Get("lifecycle.preStop.sleep.seconds")

		Expect(auth.Exists()).To(BeTrue(),
			"a registry that answers with nobody behind its token exchange still fails every pull")
		Expect(auth.Int()).To(BeNumerically(">=", registry.Int()))
	})

	It("lets the syncer release the lease at once", func() {
		Expect(containers()["syncer"].Get("lifecycle.preStop").Exists()).To(BeFalse(),
			"a wait before the release holds the whole handover back by as long")
	})

	It("fits every wait and the stop after it into the grace period", func() {
		grace := gracePeriod()
		for name, c := range containers() {
			wait := c.Get("lifecycle.preStop.sleep.seconds").Int()
			Expect(wait+stopAllowance).To(BeNumerically("<=", grace),
				"%s would be killed before it has stopped on its own", name)
		}
	})
})
