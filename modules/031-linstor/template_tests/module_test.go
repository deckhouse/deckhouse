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
  modulesImages:
    registry: registry.deckhouse.io
    registryDockercfg: Y2ZnCg==
    tags:
      common:
        alpine: be0a73850874a7000b223b461e37c32263e95574d379d4ea3305006e-1624978147531
        csiExternalAttacher119: 9b875cd5df6c7c8a27d5ae6f6fa4b90e05797857c2a2d434d90fa384-1624991865774
        csiExternalAttacher120: 53055872b801779298431ef159edc41c3270059750e41ad7ce547815-1624991942422
        csiExternalAttacher121: f553f9fe2efa329f1df967dc1ad2433db463af49675cbabeddc2eb54-1644311199940
        csiExternalAttacher122: f553f9fe2efa329f1df967dc1ad2433db463af49675cbabeddc2eb54-1644311199940
        csiExternalProvisioner119: a71ffdcb6ea573ecc44d7cbe22ba713331e92e08c8768023ecd62be0-1624991501421
        csiExternalProvisioner120: 7c94147745b7f3b3bc85cf0466e4a9d47027064cb95e0a776638d88c-1624991909639
        csiExternalProvisioner121: 2a208c02726d0fe0dad5298b2185661280664f4180968293e897cebd-1644311194495
        csiExternalProvisioner122: 2a208c02726d0fe0dad5298b2185661280664f4180968293e897cebd-1644311194495
        csiExternalResizer119: 2267cc7565514b5968ec95c99c3aca19d1930d40e2e34845f7723774-1624992194047
        csiExternalResizer120: 217459a0507b191a1744974c564df5e7de5046e5711bbafcf341a978-1624992095833
        csiExternalResizer121: e21da1d5629d422962e8dc7fce05c2a0777bc53fb7088bb71bff91b8-1644311196688
        csiExternalResizer122: e21da1d5629d422962e8dc7fce05c2a0777bc53fb7088bb71bff91b8-1644311196688
        csiExternalSnapshotter121: d554e018513a74708418c139ba57aeb75f782209d86980a729a4d97a-1644311215309
        csiExternalSnapshotter122: d554e018513a74708418c139ba57aeb75f782209d86980a729a4d97a-1644311215309
        csiLivenessprobe121: cb71a0e242c71416ee53e4e02fe0785d47c4147ee4df8bdcf0d2dfcc-1644311213440
        csiLivenessprobe122: cb71a0e242c71416ee53e4e02fe0785d47c4147ee4df8bdcf0d2dfcc-1644311213440
        csiNodeDriverRegistrar119: ecd5587d4fa58d22f91609028527b03da2f7ab9ed3c190ef90179c4b-1624991634159
        csiNodeDriverRegistrar120: 29bdbc548dd7bcab6d9cbc6156afd70870fe6818e9ea48d3a9a7eccd-1624991527573
        csiNodeDriverRegistrar121: 121aea9e665ab86dc8a83e3198acbf6a76c894c1612003c808405fb6-1638785322286
        csiNodeDriverRegistrar122: 121aea9e665ab86dc8a83e3198acbf6a76c894c1612003c808405fb6-1638785322286
        kubeRbacProxy: a4506c2aa962611cf1858c774129d2a4f233502ecc376929aa97b9f5-1639403210069
        pause: 47e1a07baefaaa885a306bb24546c929b910fe9cffffd07218b66c0a-1624979682719
      controlPlaneManager:
        controlPlaneManager: 1f38a0fe42ff3bf0f5f9f8ad64b4f40222cd4d61b3baef6f4bafedcc-1640185889591
        etcd: c47d237beff7354f150a2cd3aec929128c234b33b4b6fe938d5ab72a-1638791306829
        kubeApiserver119: 06b5098beceeac658d5341c5d80e0db913f78d07eb00ab30a5f0ebb4-1637671290445
        kubeApiserver120: a1a8cd4e196d3b911064a6116d81597766d93c26cbe341b182c5fe40-1642136416005
        kubeApiserver121: 362117904594fa9d53419d2b15147b86f267a476101d23433b2777dc-1642136422488
        kubeApiserver122: 1f4a61dacd707406cc3910850d25b9bef7ca1af7a2ff0d855c711a41-1642136422073
        kubeApiserverHealthcheck: b02dbae788d175642cac34c8756f293ef88458eb37d9223cb78c8f50-1633690869147
        kubeControllerManager119: 137373b5a99ed2521938721cb23ae3afd3e0a962511cd02617c7e5f2-1637671233259
        kubeControllerManager120: 0a59b00c342dc65bebde55398f5ad6d5bbbe7296a883bc49046b227a-1642136414827
        kubeControllerManager121: 072aaf59b94d041723cb7d9f950487860d1596066afd0eada8ff33a5-1642136450138
        kubeControllerManager122: ddb9e578e919ae6ab3092319694bb982ff0b04a650f1386bb92cbd2a-1642136474454
        kubeScheduler119: 1461eb1f0c2bfe9fb4fc5dd385cae767647a5b34930fb8833e2c61d3-1637671257076
        kubeScheduler120: f4d3608314eadfffae29f014e06309459e530694b2207ca2719d85da-1637671227093
        kubeScheduler121: 681c4418478128757e7228e8bfc63aebd02c9ae86ce608aaebf9a97e-1637671225838
        kubeScheduler122: d09a77dd6e222aa0bddd0ca86bb4a5c3a607726185a5e383af5010a5-1638785328832
      linstor:
        drbdDriverLoader: 21bc3c1a59277950aded33134152c200391b57ade099fa4e7423961d-1644342326034
        drbdReactor: 2341ed461153707d8149b6d33d32579efe2fbc3c052713053ea451f0-1644308732079
        linstorCsi: 83155bc6b3ea547f2cfc9ed392fcb0166e4abc5a5dd2eba933d308f9-1644308320933
        linstorSchedulerExtender: 81f00b8ff68814e12b378b57f990acd399482ecd6aba5a0e1679ccbf-1644280748097
        linstorServer: 68396b91baf9d4d939ad9b519a978e2e9481eaff352113b6e6007b3d-1644311161910
        linstorPoolsImporter: 06c10d03185040625eb4e27044e6d150aea7d98575b60d105d12e61f-1645450496557
        piraeusHaController: 5714ab9fe2b64540bd84514baf06bd8d9a2f63d7e4613ea37e5a8d58-1644280738598
        piraeusOperator: 8ce1396af1abafd7eee5ba256b8b8ba50ab4c2314af8400446550617-1644308310060
`
	moduleValues = `
  internal:
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
`
)

var _ = Describe("Module :: linstor :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Standard setup with SSL", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("linstor", moduleValues)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

	})
})
