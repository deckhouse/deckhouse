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
  enabledModules: ["vertical-pod-autoscaler-crd"]
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
    masterPassphrase: hackme
    httpsClientCert:
      ca: |
        -----BEGIN CERTIFICATE-----
        MIIBbTCCARSgAwIBAgIUNY8AHPMngGERxYdy9OQvB/C5Z2swCgYIKoZIzj0EAwIw
        FTETMBEGA1UEAxMKbGluc3Rvci1jYTAeFw0yMjAyMDgxOTQwMDBaFw0zMjAyMDYx
        OTQwMDBaMBUxEzARBgNVBAMTCmxpbnN0b3ItY2EwWTATBgcqhkjOPQIBBggqhkjO
        PQMBBwNCAAR/god/1bNYEJbbI4Ss3eDXxco6ztt/nTA71AcYUF0+8KaqqEgB1b4d
        h6BeqkHFtGcDLdFu4DIVlTcrsVNgzcVwo0IwQDAOBgNVHQ8BAf8EBAMCAQYwDwYD
        VR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUX/S16dEkvqDWVE3i07jOMYBxhtAwCgYI
        KoZIzj0EAwIDRwAwRAIgcNKc5Bt0Fd5z4jFL3LXyaQtQeinjYZiMcqLMrGv+NNoC
        IDJid8dT06cHhi8ltGgLZzXGw25qOu5oZSSJIRw6+QcZ
        -----END CERTIFICATE-----
      cert: |
        -----BEGIN CERTIFICATE-----
        MIIBsDCCAVWgAwIBAgIUR6gMYo0dyTWRiEKMnDYmAJeW7ZwwCgYIKoZIzj0EAwIw
        FTETMBEGA1UEAxMKbGluc3Rvci1jYTAeFw0yMjAyMDgxOTQwMDBaFw0yMzAyMDgx
        OTQwMDBaMBkxFzAVBgNVBAMTDmxpbnN0b3ItY2xpZW50MFkwEwYHKoZIzj0CAQYI
        KoZIzj0DAQcDQgAEalDjr7NfrwdjoSh1qo5vfYccFjZQxMTEy+rVH+pSEIMgp+ef
        Ipz24bDQZ/6qwZbpbiT1lywYVWDpWVxeFcV+FaN/MH0wDgYDVR0PAQH/BAQDAgWg
        MB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAMBgNVHRMBAf8EAjAAMB0G
        A1UdDgQWBBRFObmL7G6CSOLmpI2Tog79nkyzEjAfBgNVHSMEGDAWgBRf9LXp0SS+
        oNZUTeLTuM4xgHGG0DAKBggqhkjOPQQDAgNJADBGAiEAgZAQv6TBsg3PGji2u6MO
        /V46YliV5HVbtEaZG1l/10sCIQCwQOC1/9+2mOOypS6lYywJAo/l+MlbZMWITySC
        A8aK1g==
        -----END CERTIFICATE-----
      key: |
        -----BEGIN EC PRIVATE KEY-----
        MHcCAQEEIPFImbnfYGVkjAoMJrT91lAzX122Z53AXh5bFwCnNVsfoAoGCCqGSM49
        AwEHoUQDQgAEalDjr7NfrwdjoSh1qo5vfYccFjZQxMTEy+rVH+pSEIMgp+efIpz2
        4bDQZ/6qwZbpbiT1lywYVWDpWVxeFcV+FQ==
        -----END EC PRIVATE KEY-----
    httpsControllerCert:
      ca: |
        -----BEGIN CERTIFICATE-----
        MIIBbTCCARSgAwIBAgIUNY8AHPMngGERxYdy9OQvB/C5Z2swCgYIKoZIzj0EAwIw
        FTETMBEGA1UEAxMKbGluc3Rvci1jYTAeFw0yMjAyMDgxOTQwMDBaFw0zMjAyMDYx
        OTQwMDBaMBUxEzARBgNVBAMTCmxpbnN0b3ItY2EwWTATBgcqhkjOPQIBBggqhkjO
        PQMBBwNCAAR/god/1bNYEJbbI4Ss3eDXxco6ztt/nTA71AcYUF0+8KaqqEgB1b4d
        h6BeqkHFtGcDLdFu4DIVlTcrsVNgzcVwo0IwQDAOBgNVHQ8BAf8EBAMCAQYwDwYD
        VR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUX/S16dEkvqDWVE3i07jOMYBxhtAwCgYI
        KoZIzj0EAwIDRwAwRAIgcNKc5Bt0Fd5z4jFL3LXyaQtQeinjYZiMcqLMrGv+NNoC
        IDJid8dT06cHhi8ltGgLZzXGw25qOu5oZSSJIRw6+QcZ
        -----END CERTIFICATE-----
      cert: |
        -----BEGIN CERTIFICATE-----
        MIICITCCAcegAwIBAgIUCWlZHDz6YTg1qR9gCEfTZIxAs+owCgYIKoZIzj0EAwIw
        FTETMBEGA1UEAxMKbGluc3Rvci1jYTAeFw0yMjAyMDgxOTQwMDBaFw0yMzAyMDgx
        OTQwMDBaMB0xGzAZBgNVBAMTEmxpbnN0b3ItY29udHJvbGxlcjBZMBMGByqGSM49
        AgEGCCqGSM49AwEHA0IABNvORosdVjfUXIDCuJwW56pzWIWuS8nZeynHyH98bR70
        gOan5T0OnYrkOfoSNHQQaxoELAqmZNpnnv2XDxCq6J2jgewwgekwDgYDVR0PAQH/
        BAQDAgWgMB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAMBgNVHRMBAf8E
        AjAAMB0GA1UdDgQWBBQVkRsuxwJjPtt50DfgIRdUFDe9zTAfBgNVHSMEGDAWgBRf
        9LXp0SS+oNZUTeLTuM4xgHGG0DBqBgNVHREEYzBhghZsaW5zdG9yLmQ4LWxpbnN0
        b3Iuc3ZjgiRsaW5zdG9yLmQ4LWxpbnN0b3Iuc3ZjLmNsdXN0ZXIubG9jYWyCCWxv
        Y2FsaG9zdIcQAAAAAAAAAAAAAAAAAAAAAYcEfwAAATAKBggqhkjOPQQDAgNIADBF
        AiEArzqR1+ETJ4Nl1oY/uweIysaGK2IijlmC120ykYb4OZ4CIDngq4mQGgy7MVRR
        zr8cjv2Fzalj7LBvPHH58K3Pw6yS
        -----END CERTIFICATE-----
      key: |
        -----BEGIN EC PRIVATE KEY-----
        MHcCAQEEIADZ/WMhyg1cJvyqo0Eh9DzmwkgGbCrptpPU9/Bdp3LvoAoGCCqGSM49
        AwEHoUQDQgAE285Gix1WN9RcgMK4nBbnqnNYha5Lydl7KcfIf3xtHvSA5qflPQ6d
        iuQ5+hI0dBBrGgQsCqZk2mee/ZcPEKronQ==
        -----END EC PRIVATE KEY-----
    sslControllerCert:
      ca: |
        -----BEGIN CERTIFICATE-----
        MIIBbzCCARSgAwIBAgIUFW9a1I6LBhkMQtGsVZeeTu6bNJkwCgYIKoZIzj0EAwIw
        FTETMBEGA1UEAxMKbGluc3Rvci1jYTAeFw0yMjAyMDgxOTQwMDBaFw0zMjAyMDYx
        OTQwMDBaMBUxEzARBgNVBAMTCmxpbnN0b3ItY2EwWTATBgcqhkjOPQIBBggqhkjO
        PQMBBwNCAASjHMkuofBykTa+I+BjyGmWdOazCBT3mfc+j4KpvitSxLmPtUrk40Hb
        YbLsNFVPatbleWbVSOofZR9/J4RI6opco0IwQDAOBgNVHQ8BAf8EBAMCAQYwDwYD
        VR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUWi4Zj6m9hLyDB7zSim83f3a5gVwwCgYI
        KoZIzj0EAwIDSQAwRgIhALsDqkvxIZovYft7uQ26v2cRjelhOnErDq3lH0++3NHU
        AiEAmF+9IU2j0vU5pKalFBm78AkJSM+oh6nUU+Zsot/zOgQ=
        -----END CERTIFICATE-----
      cert: |
        -----BEGIN CERTIFICATE-----
        MIICIjCCAcegAwIBAgIUZxUW3TCFFR1EXLrRMlEog5pXrmUwCgYIKoZIzj0EAwIw
        FTETMBEGA1UEAxMKbGluc3Rvci1jYTAeFw0yMjAyMDgxOTQwMDBaFw0yMzAyMDgx
        OTQwMDBaMB0xGzAZBgNVBAMTEmxpbnN0b3ItY29udHJvbGxlcjBZMBMGByqGSM49
        AgEGCCqGSM49AwEHA0IABBOfhnPb10YT8iHbfmrbcdFxNV7n/l7kvLwwc3pAuodB
        nFfpvTWjo+SlCxZ2iU63sSup7a4ue4cEJf43X2enuwSjgewwgekwDgYDVR0PAQH/
        BAQDAgWgMB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAMBgNVHRMBAf8E
        AjAAMB0GA1UdDgQWBBQu5qGmDKcOXII1WKLOcfl21tR7BzAfBgNVHSMEGDAWgBRa
        LhmPqb2EvIMHvNKKbzd/drmBXDBqBgNVHREEYzBhghZsaW5zdG9yLmQ4LWxpbnN0
        b3Iuc3ZjgiRsaW5zdG9yLmQ4LWxpbnN0b3Iuc3ZjLmNsdXN0ZXIubG9jYWyCCWxv
        Y2FsaG9zdIcQAAAAAAAAAAAAAAAAAAAAAYcEfwAAATAKBggqhkjOPQQDAgNJADBG
        AiEA7gu4hK40pirmy11J3PJTUgAVvigXF2lVauYOlaFj1gMCIQC0iiAkMSYnVOkb
        MK1jHXOodUAD0JfpAmHKqNN3ky4eug==
        -----END CERTIFICATE-----
      key: |
        -----BEGIN EC PRIVATE KEY-----
        MHcCAQEEIINEYCu4qa2A5TO+Ij/zhDhNemFEqH0gnMw6SBY5uHE/oAoGCCqGSM49
        AwEHoUQDQgAEE5+Gc9vXRhPyIdt+attx0XE1Xuf+XuS8vDBzekC6h0GcV+m9NaOj
        5KULFnaJTrexK6ntri57hwQl/jdfZ6e7BA==
        -----END EC PRIVATE KEY-----
    sslNodeCert:
      ca: |
        -----BEGIN CERTIFICATE-----
        MIIBbzCCARSgAwIBAgIUFW9a1I6LBhkMQtGsVZeeTu6bNJkwCgYIKoZIzj0EAwIw
        FTETMBEGA1UEAxMKbGluc3Rvci1jYTAeFw0yMjAyMDgxOTQwMDBaFw0zMjAyMDYx
        OTQwMDBaMBUxEzARBgNVBAMTCmxpbnN0b3ItY2EwWTATBgcqhkjOPQIBBggqhkjO
        PQMBBwNCAASjHMkuofBykTa+I+BjyGmWdOazCBT3mfc+j4KpvitSxLmPtUrk40Hb
        YbLsNFVPatbleWbVSOofZR9/J4RI6opco0IwQDAOBgNVHQ8BAf8EBAMCAQYwDwYD
        VR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUWi4Zj6m9hLyDB7zSim83f3a5gVwwCgYI
        KoZIzj0EAwIDSQAwRgIhALsDqkvxIZovYft7uQ26v2cRjelhOnErDq3lH0++3NHU
        AiEAmF+9IU2j0vU5pKalFBm78AkJSM+oh6nUU+Zsot/zOgQ=
        -----END CERTIFICATE-----
      cert: |
        -----BEGIN CERTIFICATE-----
        MIIBrTCCAVOgAwIBAgIUTai7Qry7+PW/Tf3WQ70U67Bj6z8wCgYIKoZIzj0EAwIw
        FTETMBEGA1UEAxMKbGluc3Rvci1jYTAeFw0yMjAyMDgxOTQwMDBaFw0yMzAyMDgx
        OTQwMDBaMBcxFTATBgNVBAMTDGxpbnN0b3Itbm9kZTBZMBMGByqGSM49AgEGCCqG
        SM49AwEHA0IABHsqQ6YlE7qVY2C3IgXLUEPV2TCF3WsM6h+nBd3K7SgiF7h3vD2e
        Qw9dBkYONcymIJUSKa/fLsQi07qBR8FU1JCjfzB9MA4GA1UdDwEB/wQEAwIFoDAd
        BgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwDAYDVR0TAQH/BAIwADAdBgNV
        HQ4EFgQUl3VQM1LDJdIyhaT++B3drYZ+tycwHwYDVR0jBBgwFoAUWi4Zj6m9hLyD
        B7zSim83f3a5gVwwCgYIKoZIzj0EAwIDSAAwRQIhAI2Y6H+5k5yI15lx/iJfwaCI
        OJYA/15R9DQTuejyY0g+AiAGOk6ndOYZ6VNm6EsVVE6K0LfiSYI4HOXM77t5ho7r
        cA==
        -----END CERTIFICATE-----
      key: |
        -----BEGIN EC PRIVATE KEY-----
        MHcCAQEEIARKdwUIZ9nFhg1RsKhS8v6uSOq3mka9LAv919er+jtPoAoGCCqGSM49
        AwEHoUQDQgAEeypDpiUTupVjYLciBctQQ9XZMIXdawzqH6cF3crtKCIXuHe8PZ5D
        D10GRg41zKYglRIpr98uxCLTuoFHwVTUkA==
        -----END EC PRIVATE KEY-----
    webhookCert:
      ca: |
        -----BEGIN CERTIFICATE-----
        MIIBbzCCARSgAwIBAgIUFW9a1I6LBhkMQtGsVZeeTu6bNJkwCgYIKoZIzj0EAwIw
        FTETMBEGA1UEAxMKbGluc3Rvci1jYTAeFw0yMjAyMDgxOTQwMDBaFw0zMjAyMDYx
        OTQwMDBaMBUxEzARBgNVBAMTCmxpbnN0b3ItY2EwWTATBgcqhkjOPQIBBggqhkjO
        PQMBBwNCAASjHMkuofBykTa+I+BjyGmWdOazCBT3mfc+j4KpvitSxLmPtUrk40Hb
        YbLsNFVPatbleWbVSOofZR9/J4RI6opco0IwQDAOBgNVHQ8BAf8EBAMCAQYwDwYD
        VR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUWi4Zj6m9hLyDB7zSim83f3a5gVwwCgYI
        KoZIzj0EAwIDSQAwRgIhALsDqkvxIZovYft7uQ26v2cRjelhOnErDq3lH0++3NHU
        AiEAmF+9IU2j0vU5pKalFBm78AkJSM+oh6nUU+Zsot/zOgQ=
        -----END CERTIFICATE-----
      crt: |
        -----BEGIN CERTIFICATE-----
        MIIBrTCCAVOgAwIBAgIUTai7Qry7+PW/Tf3WQ70U67Bj6z8wCgYIKoZIzj0EAwIw
        FTETMBEGA1UEAxMKbGluc3Rvci1jYTAeFw0yMjAyMDgxOTQwMDBaFw0yMzAyMDgx
        OTQwMDBaMBcxFTATBgNVBAMTDGxpbnN0b3Itbm9kZTBZMBMGByqGSM49AgEGCCqG
        SM49AwEHA0IABHsqQ6YlE7qVY2C3IgXLUEPV2TCF3WsM6h+nBd3K7SgiF7h3vD2e
        Qw9dBkYONcymIJUSKa/fLsQi07qBR8FU1JCjfzB9MA4GA1UdDwEB/wQEAwIFoDAd
        BgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwDAYDVR0TAQH/BAIwADAdBgNV
        HQ4EFgQUl3VQM1LDJdIyhaT++B3drYZ+tycwHwYDVR0jBBgwFoAUWi4Zj6m9hLyD
        B7zSim83f3a5gVwwCgYIKoZIzj0EAwIDSAAwRQIhAI2Y6H+5k5yI15lx/iJfwaCI
        OJYA/15R9DQTuejyY0g+AiAGOk6ndOYZ6VNm6EsVVE6K0LfiSYI4HOXM77t5ho7r
        cA==
        -----END CERTIFICATE-----
      key: |
        -----BEGIN EC PRIVATE KEY-----
        MHcCAQEEIARKdwUIZ9nFhg1RsKhS8v6uSOq3mka9LAv919er+jtPoAoGCCqGSM49
        AwEHoUQDQgAEeypDpiUTupVjYLciBctQQ9XZMIXdawzqH6cF3crtKCIXuHe8PZ5D
        D10GRg41zKYglRIpr98uxCLTuoFHwVTUkA==
        -----END EC PRIVATE KEY-----
`
)

var _ = Describe("Module :: linstor :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Standard setup with SSL", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("linstor", moduleValues)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

	})
})
