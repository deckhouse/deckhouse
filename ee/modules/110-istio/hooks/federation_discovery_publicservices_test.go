/*
Copyright 2021 Flant CJSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	jose "github.com/square/go-jose/v3"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: federation_discovery_services ::", func() {
	f := HookExecutionConfigInit(`{
  "global":{
    "discovery":{
      "clusterUUID":"deadbeef-mycluster",
      "clusterDomain": "my.cluster"
    }
  },
  "istio":{"federation":{},"internal":{"remoteAuthnKeypair": {
    "pub":"-----BEGIN ED25519 PUBLIC KEY-----\nMCowBQYDK2VwAyEAKWjdKDeIIT4xESCMhbol662vNMpq4DxFct8GvJ500Xs=\n-----END ED25519 PUBLIC KEY-----\n",
    "priv":"-----BEGIN ED25519 PRIVATE KEY-----\nMC4CAQAwBQYDK2VwBCIEIMgNk3rr2AmIIlkKTAM9fG6+hMKvwF+pMAT3ID3M0OFK\n-----END ED25519 PRIVATE KEY-----\n"
  }}}
}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioFederation", false)

	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())

			stderrBuff := string(f.Session.Err.Contents())
			Expect(stderrBuff).To(Equal(""))
		})
	})

	Context("Empty cluster, minimal settings and federation is enabled", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.federation.enabled", true)
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())

			stderrBuff := string(f.Session.Err.Contents())
			Expect(stderrBuff).To(Equal(""))
		})
	})

	Context("Proper federations only", func() {
		BeforeEach(func() {
			f.ValuesSet(`istio.federation.enabled`, true)
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: proper-federation-0
spec:
  trustDomain: "p.f0"
  metadataEndpoint: "file:///tmp/proper-federation-0/"
status:
  metadataCache:
    public:
      clusterUUID: deadbeef-pf0
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: proper-federation-1
spec:
  trustDomain: "p.f1"
  metadataEndpoint: "file:///tmp/proper-federation-1/"
status:
  metadataCache:
    publicServices:
    - {"hostame": "some-outdated.host", "ports": [{"name": "ppp", "port": 111}]}
    public:
      clusterUUID: deadbeef-pf1
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: proper-federation-2
spec:
  trustDomain: "p.f2"
  metadataEndpoint: "file:///tmp/proper-federation-2/"
status:
  metadataCache:
    publicServices:
    - {"hostname": "some-actual.host-1", "ports": [{"name": "ppp", "port": 111}]}
    - {"hostname": "some-actual.host-2", "ports": [{"name": "ppp", "port": 111}]}
    public:
      clusterUUID: deadbeef-pf2
`))
			_ = os.MkdirAll("/tmp/proper-federation-0/private", 0755)
			ioutil.WriteFile("/tmp/proper-federation-0/private/federation-services", []byte(`
{
  "publicServices": [
    {"hostname": "a.b.c", "ports": [{"name": "ppp", "port": 123}]},
    {"hostname": "1.2.3.4", "ports": [{"name": "ppp", "port": 234}]}
  ]
}
`), 0644)
			_ = os.MkdirAll("/tmp/proper-federation-1/private", 0755)
			ioutil.WriteFile("/tmp/proper-federation-1/private/federation-services", []byte(`
{
  "publicServices": [
    {"hostname": "some-actual.host", "ports": [{"name": "ppp", "port": 111}]}
  ]
}
`), 0644)
			_ = os.MkdirAll("/tmp/proper-federation-2/private", 0755)
			ioutil.WriteFile("/tmp/proper-federation-2/private/federation-services", []byte(`
{
  "publicServices": [
    {"hostname": "some-actual.host-2", "ports": [{"name": "ppp", "port": 111}]},
    {"hostname": "some-actual.host-1", "ports": [{"name": "ppp", "port": 111}]}
  ]
}
`), 0644)

			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())

			stderrBuff := string(f.Session.Err.Contents())
			Expect(stderrBuff).To(Equal(""))

			t0, err := time.Parse(time.RFC3339, f.KubernetesGlobalResource("IstioFederation", "proper-federation-0").Field("status.metadataCache.publicServicesLastFetchTimestamp").String())
			Expect(err).ShouldNot(HaveOccurred())
			t1, err := time.Parse(time.RFC3339, f.KubernetesGlobalResource("IstioFederation", "proper-federation-1").Field("status.metadataCache.publicServicesLastFetchTimestamp").String())
			Expect(err).ShouldNot(HaveOccurred())

			Expect(t0).Should(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(t1).Should(BeTemporally("~", time.Now().UTC(), time.Minute))

			Expect(f.KubernetesGlobalResource("IstioFederation", "proper-federation-0").Field("status.metadataCache.publicServices").String()).To(MatchJSON(`
            [
              {"hostname": "1.2.3.4", "ports": [{"name": "ppp", "port": 234}]},
              {"hostname": "a.b.c", "ports": [{"name": "ppp", "port": 123}]}
            ]
`))
			Expect(f.KubernetesGlobalResource("IstioFederation", "proper-federation-1").Field("status.metadataCache.publicServices").String()).To(MatchJSON(`
            [
              {"hostname": "some-actual.host", "ports": [{"name": "ppp", "port": 111}]}
            ]
`))
			Expect(f.KubernetesGlobalResource("IstioFederation", "proper-federation-2").Field("status.metadataCache.publicServices").String()).To(MatchJSON(`
            [
              {"hostname": "some-actual.host-1", "ports": [{"name": "ppp", "port": 111}]},
              {"hostname": "some-actual.host-2", "ports": [{"name": "ppp", "port": 111}]}
            ]
`))

			tokenPF0Bytes, errpf0r := ioutil.ReadFile("/tmp/jwt-pss-proper-federation-0")
			Expect(errpf0r).ShouldNot(HaveOccurred())
			tokenPF1Bytes, errpf1r := ioutil.ReadFile("/tmp/jwt-pss-proper-federation-1")
			Expect(errpf1r).ShouldNot(HaveOccurred())
			tokenPF2Bytes, errpf2r := ioutil.ReadFile("/tmp/jwt-pss-proper-federation-2")
			Expect(errpf2r).ShouldNot(HaveOccurred())

			tokenPF0, errpf0p := jose.ParseSigned(string(tokenPF0Bytes))
			Expect(errpf0p).ShouldNot(HaveOccurred())
			tokenPF1, errpf1p := jose.ParseSigned(string(tokenPF1Bytes))
			Expect(errpf1p).ShouldNot(HaveOccurred())
			tokenPF2, errpf2p := jose.ParseSigned(string(tokenPF2Bytes))
			Expect(errpf2p).ShouldNot(HaveOccurred())

			myPubKeyPem := f.ValuesGet("istio.internal.remoteAuthnKeypair.pub").String()
			myPubKeyBlock, _ := pem.Decode([]byte(myPubKeyPem))
			myPubKey, errPubKey := x509.ParsePKIXPublicKey(myPubKeyBlock.Bytes)
			Expect(errPubKey).ShouldNot(HaveOccurred())

			tokenPF0PayloadBytes, errtpf0v := tokenPF0.Verify(myPubKey)
			Expect(errtpf0v).ShouldNot(HaveOccurred())
			tokenPF1PayloadBytes, errtpf1v := tokenPF1.Verify(myPubKey)
			Expect(errtpf1v).ShouldNot(HaveOccurred())
			tokenPF2PayloadBytes, errtpf2v := tokenPF2.Verify(myPubKey)
			Expect(errtpf2v).ShouldNot(HaveOccurred())

			var tokenPF0Payload jwtPayload
			var tokenPF1Payload jwtPayload
			var tokenPF2Payload jwtPayload

			errtpf0pmu := json.Unmarshal(tokenPF0PayloadBytes, &tokenPF0Payload)
			Expect(errtpf0pmu).ShouldNot(HaveOccurred())
			errtpf1pmu := json.Unmarshal(tokenPF1PayloadBytes, &tokenPF1Payload)
			Expect(errtpf1pmu).ShouldNot(HaveOccurred())
			errtpf2pmu := json.Unmarshal(tokenPF2PayloadBytes, &tokenPF2Payload)
			Expect(errtpf2pmu).ShouldNot(HaveOccurred())

			Expect(tokenPF0Payload.Iss).To(Equal("d8-istio"))
			Expect(tokenPF1Payload.Iss).To(Equal("d8-istio"))
			Expect(tokenPF2Payload.Iss).To(Equal("d8-istio"))

			Expect(tokenPF0Payload.Sub).To(Equal("deadbeef-mycluster"))
			Expect(tokenPF1Payload.Sub).To(Equal("deadbeef-mycluster"))
			Expect(tokenPF2Payload.Sub).To(Equal("deadbeef-mycluster"))

			Expect(tokenPF0Payload.Aud).To(Equal("deadbeef-pf0"))
			Expect(tokenPF1Payload.Aud).To(Equal("deadbeef-pf1"))
			Expect(tokenPF2Payload.Aud).To(Equal("deadbeef-pf2"))

			Expect(tokenPF0Payload.Scope).To(Equal("federation-services"))
			Expect(tokenPF1Payload.Scope).To(Equal("federation-services"))
			Expect(tokenPF2Payload.Scope).To(Equal("federation-services"))

			nbfPF0Date := time.Unix(tokenPF0Payload.Nbf, 0)
			nbfPF1Date := time.Unix(tokenPF1Payload.Nbf, 0)
			nbfPF2Date := time.Unix(tokenPF2Payload.Nbf, 0)

			expPF0Date := time.Unix(tokenPF0Payload.Exp, 0)
			expPF1Date := time.Unix(tokenPF1Payload.Exp, 0)
			expPF2Date := time.Unix(tokenPF2Payload.Exp, 0)

			Expect(nbfPF0Date).Should(BeTemporally("~", time.Now().UTC(), 25*time.Second))
			Expect(nbfPF1Date).Should(BeTemporally("~", time.Now().UTC(), 25*time.Second))
			Expect(nbfPF2Date).Should(BeTemporally("~", time.Now().UTC(), 25*time.Second))

			Expect(expPF0Date).Should(BeTemporally("~", time.Now().Add(time.Minute).UTC(), 25*time.Second))
			Expect(expPF1Date).Should(BeTemporally("~", time.Now().Add(time.Minute).UTC(), 25*time.Second))
			Expect(expPF2Date).Should(BeTemporally("~", time.Now().Add(time.Minute).UTC(), 25*time.Second))
		})
	})

	Context("Improper federation", func() {
		BeforeEach(func() {
			f.ValuesSet(`istio.federation.enabled`, true)
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: improper-federation-0
spec:
  trustDomain: "i.f0"
  metadataEndpoint: "https://some-improper-hostname-0/metadata/"
status:
  metadataCache:
    public:
      clusterUUID: deadbeef-if0
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: improper-federation-1
spec:
  trustDomain: "i.f1"
  metadataEndpoint: "https://some-improper-hostname-1/metadata/"
status: {} # no remote clusterUUID
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: improper-federation-2
spec:
  trustDomain: "my.cluster" # local clusterDomain
  metadataEndpoint: "https://some-improper-hostname-2/metadata/"
status:
  metadataCache:
    public:
      clusterUUID: deadbeef-if2
`))

			f.RunHook()
		})

		It("Hook must execute successfully with proper warnings", func() {
			Expect(f).To(ExecuteSuccessfully())

			stderrBuff := string(f.Session.Err.Contents())
			Expect(stderrBuff).Should(ContainSubstring(`ERROR: Cannot fetch public services metadata endpoint https://some-improper-hostname-0/metadata/private/federation-services for IstioFederation improper-federation-0.`))
			Expect(stderrBuff).ShouldNot(ContainSubstring(`some-improper-hostname-1`))
			Expect(stderrBuff).ShouldNot(ContainSubstring(`some-improper-hostname-2`))
		})
	})
})
