/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
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
			Expect(string(f.LoggerOutput.Contents())).To(HaveLen(0))

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
      - {"hostname": "aaa", "ports": [{"name": "ppp", "port": 123, "protocol": TCP}]}
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
      - {"hostname": "bbb", "ports": [{"name": "ppp", "port": 123, "protocol": TCP},{"name": "zzz", "port": 777, "protocol": TCP},{"name": "https-xxx", "port": 555, "protocol": TLS}]}
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
      - {"hostname": "ccc", "ports": [{"name": "ppp", "port": 123, "protocol": TCP}]}
      - {"hostname": "ddd", "ports": [{"name": "xxx", "port": 555, "protocol": TCP}]}
      - {"hostname": "eee", "ports": [{"name": "http-xxx", "port": 555, "protocol": HTTP}]}
      - {"hostname": "fff", "ports": [{"name": "https-xxx", "port": 555, "protocol": TLS}]}
      - {"hostname": "ggg", "ports": [{"name": "grpc-xxx", "port": 555, "protocol": HTTP2}]}
      - {"hostname": "hhh", "ports": [{"name": "tls-xxx", "port": 555, "protocol": TLS}]}
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
      - {"hostname": "bbb", "ports": [{"name": "ppp", "port": 123, "protocol": TCP},{"name": "zzz", "port": 777, "protocol": TCP},{"name": "grpc-xxx", "port": 555, "protocol": HTTP2},{"name": "tls-xxx", "port": 555, "protocol": TLS}]}
    public:
      clusterUUID: aaa-bbb-f5
      rootCA: abc-f5
      authnKeyPub: xyz-f5
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
            "ca": "",
            "insecureSkipVerify": false,
            "publicServices": [
              {
                "hostname": "bbb",
                "ports": [{"name": "ppp", "port": 123, "protocol": "TCP" },{"name": "zzz", "port": 777, "protocol": "TCP"},{"name": "https-xxx", "port": 555, "protocol": "TLS"}]
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
            "ca": "",
            "insecureSkipVerify": false,
            "publicServices": [
              {
                "hostname": "ccc",
                "ports": [{"name": "ppp", "port": 123, "protocol": "TCP"}]
              },
              {
                "hostname": "ddd",
                "ports": [{"name": "xxx", "port": 555, "protocol": "TCP"}]
              },
              {
                "hostname": "eee",
                "ports": [{"name": "http-xxx", "port": 555, "protocol": "HTTP"}]
              },
              {
                "hostname": "fff",
                "ports": [{"name": "https-xxx", "port": 555, "protocol": "TLS"}]
              },
              {
                "hostname": "ggg",
                "ports": [{"name": "grpc-xxx", "port": 555, "protocol": "HTTP2"}]
              },
              {
                "hostname": "hhh",
                "ports": [{"name": "tls-xxx", "port": 555, "protocol": "TLS"}]
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
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"public metadata for IstioFederation wasn't fetched yet\",\"name\":\"federation-empty\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"private metadata for IstioFederation wasn't fetched yet\",\"name\":\"federation-full-empty-ig-0\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"public metadata for IstioFederation wasn't fetched yet\",\"name\":\"federation-only-ingress\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"private metadata for IstioFederation wasn't fetched yet\",\"name\":\"federation-only-services\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"ingressGateways for IstioMulticluster weren't fetched yet\",\"name\":\"multicluster-empty-ig\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"private metadata for IstioMulticluster wasn't fetched yet\",\"name\":\"multicluster-no-apiHost\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"ingressGateways for IstioMulticluster weren't fetched yet\",\"name\":\"multicluster-no-ig\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"private metadata for IstioMulticluster wasn't fetched yet\",\"name\":\"multicluster-no-networkname\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"public metadata for IstioMulticluster wasn't fetched yet\",\"name\":\"multicluster-no-public\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"private metadata for IstioMulticluster wasn't fetched yet\",\"name\":\"multicluster-only-public\""))

			// there should be 16 log messages (including 2 new "starting token reuse logic" messages)
			Expect(strings.Split(strings.Trim(string(f.LoggerOutput.Contents()), "\n"), "\n")).To(HaveLen(16))
		})
	})

	Context("JWT Token Integration Tests", func() {
		It("Check whether a new token is being created when no secret exists.", func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: test-cluster
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://test-cluster.example.com"
status:
  metadataCache:
    private:
      ingressGateways:
      - {"address": "test-gateway", "port": 443}
      apiHost: test-cluster.example.com
      networkName: test-network
    public:
      clusterUUID: test-cluster-uuid
      rootCA: test-ca
      authnKeyPub: test-key
`))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			// Verify that a new token was generated
			multiclusters := f.ValuesGet("istio.internal.multiclusters").Array()
			Expect(multiclusters).To(HaveLen(1))

			apiJWT := f.ValuesGet("istio.internal.multiclusters.0.apiJWT").String()
			Expect(apiJWT).ToNot(BeEmpty())

			// Verify the token is valid
			validationResult := validateJWTToken(apiJWT)
			Expect(validationResult.IsExpired).To(BeFalse())
		})

		It("Checking the reuse of an existing valid secret token.", func() {
			// Create a valid JWT token
			signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: []byte("secret")}, nil)
			Expect(err).ShouldNot(HaveOccurred())

			futureTime := time.Now().Add(1 * time.Hour).Unix()
			claims := map[string]interface{}{
				"exp":   futureTime,
				"iat":   time.Now().Unix(),
				"sub":   "test-user",
				"iss":   "d8-istio",
				"aud":   "test-cluster-uuid",
				"scope": "api",
			}
			payload, err := json.Marshal(claims)
			Expect(err).ShouldNot(HaveOccurred())

			token, err := signer.Sign(payload)
			Expect(err).ShouldNot(HaveOccurred())
			validToken, err := token.CompactSerialize()
			Expect(err).ShouldNot(HaveOccurred())

			// Create kubeconfig with valid token
			validKubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://test-cluster.example.com
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: %s
`, validToken)

			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: istio-remote-secret-test-cluster
  namespace: d8-istio
  annotations:
    networking.istio.io/cluster: test-cluster
  labels:
    istio/multiCluster: "true"
data:
  test-cluster: ` + base64.StdEncoding.EncodeToString([]byte(validKubeconfig)) + `
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: test-cluster
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://test-cluster.example.com"
status:
  metadataCache:
    private:
      ingressGateways:
      - {"address": "test-gateway", "port": 443}
      apiHost: test-cluster.example.com
      networkName: test-network
    public:
      clusterUUID: test-cluster-uuid
      rootCA: test-ca
      authnKeyPub: test-key
`))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			// Verify that the existing token was reused
			multiclusters := f.ValuesGet("istio.internal.multiclusters").Array()
			Expect(multiclusters).To(HaveLen(1))

			apiJWT := f.ValuesGet("istio.internal.multiclusters.0.apiJWT").String()
			Expect(apiJWT).To(Equal(validToken))
		})

		It("Check whether a new token is being created when the existing token expires.", func() {
			// Create an expired JWT token
			signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: []byte("secret")}, nil)
			Expect(err).ShouldNot(HaveOccurred())

			pastTime := time.Now().Add(-1 * time.Hour).Unix()
			claims := map[string]interface{}{
				"exp":   pastTime,
				"iat":   time.Now().Add(-2 * time.Hour).Unix(),
				"sub":   "test-user",
				"iss":   "d8-istio",
				"aud":   "test-cluster-uuid",
				"scope": "api",
			}
			payload, err := json.Marshal(claims)
			Expect(err).ShouldNot(HaveOccurred())

			token, err := signer.Sign(payload)
			Expect(err).ShouldNot(HaveOccurred())
			expiredToken, err := token.CompactSerialize()
			Expect(err).ShouldNot(HaveOccurred())

			// Create kubeconfig with expired token
			expiredKubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://test-cluster.example.com
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: %s
`, expiredToken)

			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: istio-remote-secret-test-cluster
  namespace: d8-istio
  annotations:
    networking.istio.io/cluster: test-cluster
  labels:
    istio/multiCluster: "true"
data:
  test-cluster: ` + base64.StdEncoding.EncodeToString([]byte(expiredKubeconfig)) + `
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: test-cluster
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://test-cluster.example.com"
status:
  metadataCache:
    private:
      ingressGateways:
      - {"address": "test-gateway", "port": 443}
      apiHost: test-cluster.example.com
      networkName: test-network
    public:
      clusterUUID: test-cluster-uuid
      rootCA: test-ca
      authnKeyPub: test-key
`))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			// Verify that a new token was generated (different from expired one)
			multiclusters := f.ValuesGet("istio.internal.multiclusters").Array()
			Expect(multiclusters).To(HaveLen(1))

			apiJWT := f.ValuesGet("istio.internal.multiclusters.0.apiJWT").String()
			Expect(apiJWT).ToNot(Equal(expiredToken))
			Expect(apiJWT).ToNot(BeEmpty())

			// Verify the new token is valid
			validationResult := validateJWTToken(apiJWT)
			Expect(validationResult.IsExpired).To(BeFalse())
		})
	})
})
