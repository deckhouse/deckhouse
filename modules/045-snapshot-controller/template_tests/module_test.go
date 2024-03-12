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

package template_tests

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	globalValues = `
  enabledModules: ["vertical-pod-autoscaler-crd", "sds-replicated-volume"]
  highAvailability: true
  modules:
    placement: {}
  discovery:
    kubernetesVersion: 1.21.9
    d8SpecificNodeCountByRole:
      worker: 3
      master: 3
`
	moduleValues = `
  internal:
    webhookCert:
      ca: |
        -----BEGIN CERTIFICATE-----
        MIIBlzCCATygAwIBAgIUQOyRzwuWMLXJ9GhWR7ITjvfV4vcwCgYIKoZIzj0EAwIw
        KTEnMCUGA1UEAxMec25hcHNob3QtdmFsaWRhdGlvbi13ZWJob29rLWNhMB4XDTIy
        MDMxOTAwMDIwMFoXDTMyMDMxNjAwMDIwMFowKTEnMCUGA1UEAxMec25hcHNob3Qt
        dmFsaWRhdGlvbi13ZWJob29rLWNhMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE
        undq9sD1t94g+jcDFReqCD89h3wl1zWwNVRVbVvEaPjM/6i5edzZu8Z9JSfEX+zD
        wgmd6YMcCM1DUzcQJOWP6qNCMEAwDgYDVR0PAQH/BAQDAgEGMA8GA1UdEwEB/wQF
        MAMBAf8wHQYDVR0OBBYEFGFBc39cegiUHL9jZBRnIZyaSB5dMAoGCCqGSM49BAMC
        A0kAMEYCIQCvcRBPvX9lGJ4ZV5W8cvZRBWpJ+sMEW4CyY/BtaQ94hAIhAJ/YnC1y
        ZsNK+wwldkAEwZS/kKN/ny8EuRkJlEsA368w
        -----END CERTIFICATE-----
      cert: |
        -----BEGIN CERTIFICATE-----
        MIICSjCCAfCgAwIBAgIUD7SVLWA+oj2GOtybppwftTS5odswCgYIKoZIzj0EAwIw
        KTEnMCUGA1UEAxMec25hcHNob3QtdmFsaWRhdGlvbi13ZWJob29rLWNhMB4XDTIy
        MDMxOTAwMDIwMFoXDTMyMDMxNjAwMDIwMFowJjEkMCIGA1UEAxMbc25hcHNob3Qt
        dmFsaWRhdGlvbi13ZWJob29rMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE87kP
        JEKnx4gItEwd/U54klN9jUfSiNtOYo0UVpTuYbU/YTTawP6PTV3UctrzQEdWPLJ9
        73UyJGWTh6Dsc9TdEaOB+DCB9TAOBgNVHQ8BAf8EBAMCBaAwHQYDVR0lBBYwFAYI
        KwYBBQUHAwEGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAwHQYDVR0OBBYEFGsV/7M0
        WlEABZoWjuRFsduY9E7TMB8GA1UdIwQYMBaAFGFBc39cegiUHL9jZBRnIZyaSB5d
        MHYGA1UdEQRvMG2CG3NuYXBzaG90LXZhbGlkYXRpb24td2ViaG9va4Irc25hcHNo
        b3QtdmFsaWRhdGlvbi13ZWJob29rLmt1YmUtc3lzdGVtLnN2Y4IJbG9jYWxob3N0
        hxAAAAAAAAAAAAAAAAAAAAABhwR/AAABMAoGCCqGSM49BAMCA0gAMEUCIQDgSOEJ
        oqqceIEKDc5EGiHSQLHY+9z7XuVr1lgeFkYrDgIgA2Vcxj/sLMQGy+/ilnwqHlQ2
        GfYruDyyCXu0Oh1SDqM=
        -----END CERTIFICATE-----
      key: |
        -----BEGIN EC PRIVATE KEY-----
        MHcCAQEEICoFPX2z3Dd9KjslJjdJaVtRdN2fXkOuEB9pjxDkugo3oAoGCCqGSM49
        AwEHoUQDQgAE87kPJEKnx4gItEwd/U54klN9jUfSiNtOYo0UVpTuYbU/YTTawP6P
        TV3UctrzQEdWPLJ973UyJGWTh6Dsc9TdEQ==
        -----END EC PRIVATE KEY-----
`
)

var _ = Describe("Module :: snapshot-controller :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Standard setup with SSL", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("snapshotController", moduleValues)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

	})
})
