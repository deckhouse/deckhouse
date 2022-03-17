/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	jose "github.com/square/go-jose/v3"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: alliance_metadata_merge ::", func() {
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

	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			Expect(f.ValuesGet("istio.internal.federations").String()).To(MatchJSON(`[]`))
			Expect(f.ValuesGet("istio.internal.multiclusters").String()).To(MatchJSON(`[]`))
			Expect(f.ValuesGet("istio.internal.remotePublicMetadata").String()).To(MatchJSON(`{}`))
			Expect(f.ValuesGet("istio.internal.multiclustersNeedIngressGateway").Bool()).To(BeFalse())
		})
	})

	Context("Federations and Multiclusters with different cache fullfillment", func() {
		BeforeEach(func() {
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
    private:
      ingressGateways:
      - {"address": "aaa", "port": 222}
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
    private:
      publicServices:
      - {"hostname": "aaa", "ports": [{"name": "ppp", "port": 123}], "virtualIP": "169.0.0.0"}
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
    private:
      ingressGateways:
      - {"address": "bbb", "port": 222}
      publicServices:
      - {"hostname": "bbb", "ports": [{"name": "ppp", "port": 123},{"name": "zzz", "port": 777}], "virtualIP": "169.0.0.1"}
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
    private:
      ingressGateways:
      - {"address": "ccc", "port": 222}
      publicServices:
      - {"hostname": "ccc", "ports": [{"name": "ppp", "port": 123}], "virtualIP": "169.0.0.2"}
      - {"hostname": "ddd", "ports": [{"name": "xxx", "port": 555}], "virtualIP": "169.0.0.3"}
    public:
      clusterUUID: aaa-bbb-f4
      rootCA: abc-f4
      authnKeyPub: xyz-f4
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: federation-full-empty-ig-0
spec:
  trustDomain: "f5"
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    private:
      ingressGateways: []
      publicServices:
      - {"hostname": "bbb", "ports": [{"name": "ppp", "port": 123},{"name": "zzz", "port": 777}], "virtualIP": "169.0.0.3"}
    public:
      clusterUUID: aaa-bbb-f5
      rootCA: abc-f5
      authnKeyPub: xyz-f5
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: federation-full-no-virtualip-0
spec:
  trustDomain: "f5"
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    private:
      ingressGateways:
      - {"address": "ccc", "port": 222}
      publicServices:
      - {"hostname": "ccc", "ports": [{"name": "ppp", "port": 123}], "virtualIP": "169.0.0.2"}
      - {"hostname": "ddd", "ports": [{"name": "xxx", "port": 555}]} # no virtualIP, federation should be skipped
    public:
      clusterUUID: aaa-bbb-f4
      rootCA: abc-f4
      authnKeyPub: xyz-f4
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: multicluster-full-0
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    private:
      ingressGateways:
      - {"address": "ddd", "port": 333}
      apiHost: istio-api-0.example.com
      networkName: network-qqq-123
    public:
      clusterUUID: aaa-bbb-m0
      rootCA: abc-m0
      authnKeyPub: xyz-m0
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: multicluster-full-1
spec:
  enableIngressGateway: false
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    private:
      apiHost: istio-api-1.example.com
      networkName: network-xxx-123
    public:
      clusterUUID: aaa-bbb-m1
      rootCA: abc-m1
      authnKeyPub: xyz-m1
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: multicluster-only-public
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    public:
      clusterUUID: aaa-bbb-m2
      rootCA: abc-m2
      authnKeyPub: xyz-m2
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: multicluster-no-ig
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    private:
      apiHost: istio-api.example.com
      networkName: network-qqq-123
    public:
      clusterUUID: aaa-bbb-m3
      rootCA: abc-m3
      authnKeyPub: xyz-m3
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: multicluster-empty-ig
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    private:
      ingressGateways: []
      apiHost: istio-api.example.com
      networkName: network-qqq-123
    public:
      clusterUUID: aaa-bbb-m4
      rootCA: abc-m4
      authnKeyPub: xyz-m4
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: multicluster-no-apiHost
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    private:
      ingressGateways:
      - {"address": "ddd", "port": 333}
      networkName: network-qqq-123
    public:
      clusterUUID: aaa-bbb-m5
      rootCA: abc-m5
      authnKeyPub: xyz-m5
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: multicluster-no-networkname
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    private:
      ingressGateways:
      - {"address": "ddd", "port": 333}
      apiHost: istio-api.example.com
    public:
      clusterUUID: aaa-bbb-m6
      rootCA: abc-m6
      authnKeyPub: xyz-m6
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: multicluster-no-public
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    private:
      ingressGateways:
      - {"address": "ddd", "port": 333}
      apiHost: istio-api.example.com
      networkName: network-qqq-123
`))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("istio.internal.federations").String()).To(MatchJSON(`
[
          {
            "ingressGateways": [
              {
                "address": "bbb",
                "port": 222
              }
            ],
            "name": "federation-only-full-0",
            "publicServices": [
              {
                "hostname": "bbb",
                "virtualIP": "169.0.0.1",
                "ports": [{"name": "ppp", "port": 123},{"name": "zzz", "port": 777}]
              }
            ],
            "spiffeEndpoint": "https://some-proper-host/public/spiffe-bundle-endpoint",
            "trustDomain": "f3"
          },
          {
            "ingressGateways": [
              {
                "address": "ccc",
                "port": 222
              }
            ],
            "name": "federation-only-full-1",
            "publicServices": [
              {
                "hostname": "ccc",
                "virtualIP": "169.0.0.2",
                "ports": [{"name": "ppp", "port": 123}]
              },
              {
                "hostname": "ddd",
                "virtualIP": "169.0.0.3",
                "ports": [{"name": "xxx", "port": 555}]
              }
            ],
            "spiffeEndpoint": "https://some-proper-host/public/spiffe-bundle-endpoint",
            "trustDomain": "f4"
          }
        ]
`))

			Expect(f.ValuesGet("istio.internal.multiclusters.0.name").String()).To(Equal("multicluster-full-0"))
			Expect(f.ValuesGet("istio.internal.multiclusters.0.spiffeEndpoint").String()).To(Equal("https://some-proper-host/public/spiffe-bundle-endpoint"))
			Expect(f.ValuesGet("istio.internal.multiclusters.0.apiHost").String()).To(Equal("istio-api-0.example.com"))
			Expect(f.ValuesGet("istio.internal.multiclusters.0.networkName").String()).To(Equal("network-qqq-123"))
			Expect(f.ValuesGet("istio.internal.multiclusters.0.ingressGateways").String()).To(MatchJSON(`
[
  {
    "address": "ddd",
    "port": 333
  }
]
`))
			Expect(f.ValuesGet("istio.internal.multiclusters.1.name").String()).To(Equal("multicluster-full-1"))
			Expect(f.ValuesGet("istio.internal.multiclusters.1.spiffeEndpoint").String()).To(Equal("https://some-proper-host/public/spiffe-bundle-endpoint"))
			Expect(f.ValuesGet("istio.internal.multiclusters.1.apiHost").String()).To(Equal("istio-api-1.example.com"))
			Expect(f.ValuesGet("istio.internal.multiclusters.1.networkName").String()).To(Equal("network-xxx-123"))
			Expect(f.ValuesGet("istio.internal.multiclusters.1.ingressGateways").Exists()).To(BeTrue())
			Expect(f.ValuesGet("istio.internal.multiclusters.1.ingressGateways").Value()).To(BeNil())

			Expect(f.ValuesGet("istio.internal.multiclusters.2").Exists()).To(BeFalse())

			Expect(f.ValuesGet("istio.internal.multiclustersNeedIngressGateway").Bool()).To(BeTrue())

			tokenM0String := f.ValuesGet("istio.internal.multiclusters.0.apiJWT").String()

			tokenM0, errm0p := jose.ParseSigned(tokenM0String)
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
			Expect(expM0Date).Should(BeTemporally("~", time.Now().Add(time.Hour*24*366).UTC(), 25*time.Second))

			Expect(f.ValuesGet("istio.internal.remotePublicMetadata").String()).To(MatchJSON(`
		{
		  "aaa-bbb-f2": {"clusterUUID": "aaa-bbb-f2", "rootCA": "abc-f2", "authnKeyPub": "xyz-f2"},
		  "aaa-bbb-f3": {"clusterUUID": "aaa-bbb-f3", "rootCA": "abc-f3", "authnKeyPub": "xyz-f3"},
		  "aaa-bbb-f4": {"clusterUUID": "aaa-bbb-f4", "rootCA": "abc-f4", "authnKeyPub": "xyz-f4"},
		  "aaa-bbb-f5": {"clusterUUID": "aaa-bbb-f5", "rootCA": "abc-f5", "authnKeyPub": "xyz-f5"},
		  "aaa-bbb-m0": {"clusterUUID": "aaa-bbb-m0", "rootCA": "abc-m0", "authnKeyPub": "xyz-m0"},
		  "aaa-bbb-m1": {"clusterUUID": "aaa-bbb-m1", "rootCA": "abc-m1", "authnKeyPub": "xyz-m1"},
		  "aaa-bbb-m2": {"clusterUUID": "aaa-bbb-m2", "rootCA": "abc-m2", "authnKeyPub": "xyz-m2"},
		  "aaa-bbb-m3": {"clusterUUID": "aaa-bbb-m3", "rootCA": "abc-m3", "authnKeyPub": "xyz-m3"},
		  "aaa-bbb-m4": {"clusterUUID": "aaa-bbb-m4", "rootCA": "abc-m4", "authnKeyPub": "xyz-m4"},
		  "aaa-bbb-m5": {"clusterUUID": "aaa-bbb-m5", "rootCA": "abc-m5", "authnKeyPub": "xyz-m5"},
		  "aaa-bbb-m6": {"clusterUUID": "aaa-bbb-m6", "rootCA": "abc-m6", "authnKeyPub": "xyz-m6"}
		}
`))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("public metadata for IstioFederation federation-empty wasn't fetched yet"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("private metadata for IstioFederation federation-full-empty-ig-0 wasn't fetched yet"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("virtualIP wasn't set for publicService ddd of IstioFederation federation-full-no-virtualip-0"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("public metadata for IstioFederation federation-only-ingress wasn't fetched yet"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("private metadata for IstioFederation federation-only-services wasn't fetched yet"))

			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("ingressGateways for IstioMulticluster multicluster-empty-ig weren't fetched yet"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("private metadata for IstioMulticluster multicluster-no-apiHost wasn't fetched yet"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("ingressGateways for IstioMulticluster multicluster-no-ig weren't fetched yet"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("private metadata for IstioMulticluster multicluster-no-networkname wasn't fetched yet"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("public metadata for IstioMulticluster multicluster-no-public wasn't fetched yet"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("private metadata for IstioMulticluster multicluster-only-public wasn't fetched yet"))

			// there should be 11 log messages
			Expect(strings.Split(strings.Trim(string(f.LogrusOutput.Contents()), "\n"), "\n")).To(HaveLen(11))
		})
	})
})
