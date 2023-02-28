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

package snapshot

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Modules :: upmeter :: hooks :: smokemini :: indexing ::", func() {
	It("converts index to Pod name", func() {
		Expect(Index("a").PodName()).To(Equal("smoke-mini-a-0"))
		Expect(Index("d").PodName()).To(Equal("smoke-mini-d-0"))
	})

	It("converts index to PVC name", func() {
		Expect(Index("a").PersistenceVolumeClaimName()).To(Equal("disk-smoke-mini-a-0"))
		Expect(Index("d").PersistenceVolumeClaimName()).To(Equal("disk-smoke-mini-d-0"))
	})

	It("parses index from Pod name", func() {
		Expect(IndexFromPodName("smoke-mini-a-0").String()).To(Equal("a"))
		Expect(IndexFromPodName("smoke-mini-d-0").String()).To(Equal("d"))
	})

	It("parses index from PVC name", func() {
		Expect(IndexFromPVCName("disk-smoke-mini-a-0").String()).To(Equal("a"))
		Expect(IndexFromPVCName("disk-smoke-mini-d-0").String()).To(Equal("d"))
	})
})
