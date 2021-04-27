package hooks

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	jose "github.com/square/go-jose/v3"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: remote_clusters_metadata_merge ::", func() {
	f := HookExecutionConfigInit(`{
  "global":{
    "discovery":{
      "clusterUUID":"deadbeef-mycluster",
      "clusterDomain": "my.cluster"
    }
  },
  "istio":{"internal":{"remoteAuthnKeypair": {
    "pub":"-----BEGIN ED25519 PUBLIC KEY-----\nMCowBQYDK2VwAyEAKWjdKDeIIT4xESCMhbol662vNMpq4DxFct8GvJ500Xs=\n-----END ED25519 PUBLIC KEY-----\n",
    "priv":"-----BEGIN ED25519 PRIVATE KEY-----\nMC4CAQAwBQYDK2VwBCIEIMgNk3rr2AmIIlkKTAM9fG6+hMKvwF+pMAT3ID3M0OFK\n-----END ED25519 PRIVATE KEY-----\n"
  }}}
}`, "")

	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioFederation", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioMulticluster", false)

	Context("Proper federations only", func() {
		BeforeEach(func() {
			f.ValuesSet(`istio.federation.enabled`, true)
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: federation-empty
spec:
  trustDomain: "f0"
  metadataEndpoint: "https://some-proper-host/"
status: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: federation-only-ingress
spec:
  trustDomain: "f1"
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    ingressGateways:
    - {"address": "aaa", "port": 123}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: federation-only-services
spec:
  trustDomain: "f2"
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    publicServices:
    - {"hostname": "aaa", "port": 123}
    public:
      clusterUUID: aaa-bbb-f2
      rootCA: abc-f2
      authnKeyPub: xyz-f2
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: federation-only-full-0
spec:
  trustDomain: "f3"
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    ingressGateways:
    - {"address": "bbb", "port": 123}
    publicServices:
    - {"hostname": "bbb", "port": 123}
    public:
      clusterUUID: aaa-bbb-f3
      rootCA: abc-f3
      authnKeyPub: xyz-f3
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: federation-only-full-1
spec:
  trustDomain: "f4"
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    ingressGateways:
    - {"address": "ccc", "port": 123}
    publicServices:
    - {"hostname": "ccc", "port": 123}
    public:
      clusterUUID: aaa-bbb-f4
      rootCA: abc-f4
      authnKeyPub: xyz-f4
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: multicluster-0
spec:
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    apiHost: istio-api.example.com
    public:
      clusterUUID: aaa-bbb-m0
      rootCA: abc-m0
      authnKeyPub: xyz-m0
`))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())

			stderrBuff := string(f.Session.Err.Contents())
			Expect(stderrBuff).To(Equal(""))

			Expect(f.ValuesGet("istio.internal.federations").String()).To(MatchJSON(`
[
          {
            "ingressGateways": [
              {
                "address": "bbb",
                "port": 123
              }
            ],
            "name": "federation-only-full-0",
            "publicServices": [
              {
                "hostname": "bbb",
                "port": 123
              }
            ],
            "spiffeEndpoint": "https://some-proper-host/public/spiffe-bundle-endpoint",
            "trustDomain": "f3"
          },
          {
            "ingressGateways": [
              {
                "address": "ccc",
                "port": 123
              }
            ],
            "name": "federation-only-full-1",
            "publicServices": [
              {
                "hostname": "ccc",
                "port": 123
              }
            ],
            "spiffeEndpoint": "https://some-proper-host/public/spiffe-bundle-endpoint",
            "trustDomain": "f4"
          }
        ]
`))

			Expect(f.ValuesGet("istio.internal.multiclusters.0.name").String()).To(Equal("multicluster-0"))
			Expect(f.ValuesGet("istio.internal.multiclusters.0.spiffeEndpoint").String()).To(Equal("https://some-proper-host/public/spiffe-bundle-endpoint"))
			Expect(f.ValuesGet("istio.internal.multiclusters.0.apiHost").String()).To(Equal("istio-api.example.com"))

			tokenM0Bytes, errm0r := ioutil.ReadFile("/tmp/jwt-api-multicluster-0")
			Expect(errm0r).ShouldNot(HaveOccurred())

			tokenM0, errm0p := jose.ParseSigned(string(tokenM0Bytes))
			Expect(errm0p).ShouldNot(HaveOccurred())

			myPubKeyPem := f.ValuesGet("istio.internal.remoteAuthnKeypair.pub").String()
			myPubKeyBlock, _ := pem.Decode([]byte(myPubKeyPem))
			myPubKey, errPubKey := x509.ParsePKIXPublicKey(myPubKeyBlock.Bytes)
			Expect(errPubKey).ShouldNot(HaveOccurred())

			tokenM0PayloadBytes, errtm0v := tokenM0.Verify(myPubKey)
			Expect(errtm0v).ShouldNot(HaveOccurred())

			var tokenM0Payload jwtPayload
			errtm0pmu := json.Unmarshal(tokenM0PayloadBytes, &tokenM0Payload)
			Expect(errtm0pmu).ShouldNot(HaveOccurred())

			Expect(tokenM0Payload.Iss).To(Equal("d8-istio"))
			Expect(tokenM0Payload.Sub).To(Equal("deadbeef-mycluster"))
			Expect(tokenM0Payload.Aud).To(Equal("aaa-bbb-m0"))
			Expect(tokenM0Payload.Scope).To(Equal("api"))

			nbfM0Date := time.Unix(tokenM0Payload.Nbf, 0)
			expM0Date := time.Unix(tokenM0Payload.Exp, 0)

			Expect(nbfM0Date).Should(BeTemporally("~", time.Now().UTC(), 25*time.Second))
			Expect(expM0Date).Should(BeTemporally("~", time.Now().Add(8760*time.Hour).UTC(), 25*time.Second))

			Expect(f.ValuesGet("istio.internal.remotePublicMetadata").String()).To(MatchJSON(`
		{
		  "aaa-bbb-f2": {"rootCA": "abc-f2", "authnKeyPub": "xyz-f2"},
		  "aaa-bbb-f3": {"rootCA": "abc-f3", "authnKeyPub": "xyz-f3"},
		  "aaa-bbb-f4": {"rootCA": "abc-f4", "authnKeyPub": "xyz-f4"},
		  "aaa-bbb-m0": {"rootCA": "abc-m0", "authnKeyPub": "xyz-m0"}
		}
`))
		})
	})
})
