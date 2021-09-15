/*
Copyright 2021 Flant JSC
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

var _ = Describe("Istio hooks :: multicluster_discovery_ingressgateways ::", func() {
	f := HookExecutionConfigInit(`{
  "global":{
    "discovery":{
      "clusterUUID":"deadbeef-mycluster",
      "clusterDomain": "my.cluster"
    }
  },
  "istio":{"multicluster":{},"internal":{"remoteAuthnKeypair": {
    "pub":"-----BEGIN ED25519 PUBLIC KEY-----\nMCowBQYDK2VwAyEAKWjdKDeIIT4xESCMhbol662vNMpq4DxFct8GvJ500Xs=\n-----END ED25519 PUBLIC KEY-----\n",
    "priv":"-----BEGIN ED25519 PRIVATE KEY-----\nMC4CAQAwBQYDK2VwBCIEIMgNk3rr2AmIIlkKTAM9fG6+hMKvwF+pMAT3ID3M0OFK\n-----END ED25519 PRIVATE KEY-----\n"
  }}}
}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioMulticluster", false)

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

	Context("Empty cluster, minimal settings and multicluster is enabled", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.multicluster.enabled", true)
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())

			stderrBuff := string(f.Session.Err.Contents())
			Expect(stderrBuff).To(Equal(""))
		})
	})

	Context("Proper multiclusters only", func() {
		BeforeEach(func() {
			f.ValuesSet(`istio.multicluster.enabled`, true)
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: proper-multicluster-0
spec:
  enableIngressGateway: true
  metadataEndpoint: "file:///tmp/proper-multicluster-0/"
status:
  metadataCache:
    public:
      clusterUUID: deadbeef-pf0
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: proper-multicluster-1
spec:
  enableIngressGateway: true
  metadataEndpoint: "file:///tmp/proper-multicluster-1/"
status:
  metadataCache:
    ingressGateways:
    - {"address": "some-outdated.host", "port": 111}
    public:
      clusterUUID: deadbeef-pf1
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: proper-multicluster-2
spec:
  enableIngressGateway: true
  metadataEndpoint: "file:///tmp/proper-multicluster-2/"
status:
  metadataCache:
    ingressGateways:
    - {"address": "some-actual.host-1", "port": 111}
    - {"address": "some-actual.host-2", "port": 111}
    public:
      clusterUUID: deadbeef-pf2
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: proper-multicluster-3
spec:
  enableIngressGateway: false
  metadataEndpoint: "file:///tmp/proper-multicluster-3/"
status:
  metadataCache:
    public:
      clusterUUID: deadbeef-pf3

`))
			_ = os.MkdirAll("/tmp/proper-multicluster-0/private/", 0755)
			ioutil.WriteFile("/tmp/proper-multicluster-0/private/alliance-ingressgateways", []byte(`
{
  "ingressGateways": [
    {"address": "a.b.c", "port": 123},
    {"address": "1.2.3.4", "port": 234}
  ]
}
`), 0644)
			_ = os.MkdirAll("/tmp/proper-multicluster-1/private", 0755)
			ioutil.WriteFile("/tmp/proper-multicluster-1/private/alliance-ingressgateways", []byte(`
{
  "ingressGateways": [
    {"address": "some-actual.host", "port": 111}
  ]
}
`), 0644)
			_ = os.MkdirAll("/tmp/proper-multicluster-2/private", 0755)
			ioutil.WriteFile("/tmp/proper-multicluster-2/private/alliance-ingressgateways", []byte(`
{
  "ingressGateways": [
    {"address": "some-actual.host-2", "port": 111},
    {"address": "some-actual.host-1", "port": 111}
  ]
}
`), 0644)

			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())

			stderrBuff := string(f.Session.Err.Contents())
			Expect(stderrBuff).To(Equal(""))

			t0, err := time.Parse(time.RFC3339, f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-0").Field("status.metadataCache.ingressGatewaysLastFetchTimestamp").String())
			Expect(err).ShouldNot(HaveOccurred())
			t1, err := time.Parse(time.RFC3339, f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-1").Field("status.metadataCache.ingressGatewaysLastFetchTimestamp").String())
			Expect(err).ShouldNot(HaveOccurred())

			Expect(t0).Should(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(t1).Should(BeTemporally("~", time.Now().UTC(), time.Minute))

			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-0").Field("status.metadataCache.ingressGateways").String()).To(MatchJSON(`
            [
              {"address": "1.2.3.4", "port": 234},
              {"address": "a.b.c", "port": 123}
            ]
`))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-1").Field("status.metadataCache.ingressGateways").String()).To(MatchJSON(`
            [
              {"address": "some-actual.host", "port": 111}
            ]
`))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-2").Field("status.metadataCache.ingressGateways").String()).To(MatchJSON(`
            [
              {"address": "some-actual.host-1", "port": 111},
              {"address": "some-actual.host-2", "port": 111}
            ]
`))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-3").Field("status.metadataCache.ingressGateways").Exists()).To(BeFalse())

			tokenPF0Bytes, errpf0r := ioutil.ReadFile("/tmp/jwt-igs-proper-multicluster-0")
			Expect(errpf0r).ShouldNot(HaveOccurred())
			tokenPF1Bytes, errpf1r := ioutil.ReadFile("/tmp/jwt-igs-proper-multicluster-1")
			Expect(errpf1r).ShouldNot(HaveOccurred())
			tokenPF2Bytes, errpf2r := ioutil.ReadFile("/tmp/jwt-igs-proper-multicluster-2")
			Expect(errpf2r).ShouldNot(HaveOccurred())

			Expect("/tmp/jwt-igs-proper-multicluster-3").To(Not(BeAnExistingFile()))

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

			Expect(tokenPF0Payload.Scope).To(Equal("alliance-ingressgateways"))
			Expect(tokenPF1Payload.Scope).To(Equal("alliance-ingressgateways"))
			Expect(tokenPF2Payload.Scope).To(Equal("alliance-ingressgateways"))

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

	Context("Improper multicluster", func() {
		BeforeEach(func() {
			f.ValuesSet(`istio.multicluster.enabled`, true)
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: improper-multicluster-0
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://some-improper-hostname-0/metadata/"
status:
  metadataCache:
    public:
      clusterUUID: deadbeef-if0
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: improper-multicluster-1
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://some-improper-hostname-1/metadata/"
status: {} # no remote clusterUUID
`))

			f.RunHook()
		})

		It("Hook must execute successfully with proper warnings", func() {
			Expect(f).To(ExecuteSuccessfully())

			stderrBuff := string(f.Session.Err.Contents())
			Expect(stderrBuff).Should(ContainSubstring(`ERROR: Cannot fetch ingressgateways metadata endpoint https://some-improper-hostname-0/metadata/private/alliance-ingressgateways for IstioMulticluster improper-multicluster-0.`))
			Expect(stderrBuff).ShouldNot(ContainSubstring(`some-improper-hostname-1`))
		})
	})
})
