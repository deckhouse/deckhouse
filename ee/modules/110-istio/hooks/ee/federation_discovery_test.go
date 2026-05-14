/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/square/go-jose/v3"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: federation_discovery ::", func() {
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
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LoggerOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0].Action).Should(Equal(operation.ActionExpireMetrics))
		})
	})

	Context("Empty cluster, minimal settings and federation is enabled", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.federation.enabled", true)
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LoggerOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0].Action).Should(Equal(operation.ActionExpireMetrics))
		})
	})

	Context("Proper federations only", func() {
		var bearerTokens = map[string]string{}

		BeforeEach(func() {
			f.ValuesSet(`istio.federation.enabled`, true)
			f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: proper-federation-0
spec:
  trustDomain: "p.f0"
  metadataEndpoint: "https://proper-hostname-0/metadata/"
status: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: proper-federation-1
spec:
  trustDomain: "p.f1"
  metadataEndpoint: "https://proper-hostname-1/metadata/"
status:
  metadataCache:
    private:
      ingressGateways:
      - {"address": "some-outdated.host", "port": 111} # must be overwritten by the new data
      publicServices:
      - {"hostame": "some-outdated.host", "ports": [{"name": "ppp", "port": 111}]} # must be overwritten by the new data
    public:
      clusterUUID: bad-cluster-uuid # should be changed
      rootCA: bad-root-ca
      authnKeyPub: bad-authn-key-pub
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: proper-federation-2
spec:
  trustDomain: "p.f2"
  metadataEndpoint: "https://proper-hostname-2/metadata/"
status:
  metadataCache:
    private:
      ingressGateways:
      - {"address": "some-actual.host-1", "port": 111} # should be saved
      - {"address": "some-outdatad.host-2", "port": 111} # should be deleted
      publicServices:
      - {"hostname": "some-actual.host-1", "ports": [{"name": "ppp", "port": 111}]} # should be saved
      - {"hostname": "some-outdated.host-2", "ports": [{"name": "ppp", "port": 111}]} # should be deleted
      - {"hostname": "some-actual.host-3", "ports": [{"name": "ppp", "port": 111}]} # port should be changed to 222
      - {"hostname": "some-actual.host-4", "ports": [{"name": "https-ppp", "port": 111}]} # port should be changed to 222
      - {"hostname": "some-actual.host-5", "ports": [{"name": "grps-ppp", "port": 222}]} # port should be changed to 222
`)
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))

			respMap := map[string]map[string]HTTPMockResponse{
				"proper-hostname-0": {
					"/metadata/public/public.json": {
						Response: `{
						  "clusterUUID": "proper-uuid-0",
						  "authnKeyPub": "proper-authn-0",
						  "rootCA": "proper-root-ca-0"
						}`,
						Code: http.StatusOK,
					},
					"/metadata/private/federation.json": {
						Response: `{
						  "ingressGateways": [
							{"address": "a.b.c", "port": 123},
							{"address": "1.2.3.4", "port": 234}
						  ],
						  "publicServices": [
							{"hostname": "a.b.c", "ports": [{"name": "ppp", "port": 123}]},
							{"hostname": "1.2.3.4", "ports": [{"name": "ppp", "port": 234}]}
						  ]
						}`,
						Code: http.StatusOK,
					},
				},
				"proper-hostname-1": {
					"/metadata/public/public.json": {
						Response: `{
						  "clusterUUID": "proper-uuid-1",
						  "authnKeyPub": "proper-authn-1",
						  "rootCA": "proper-root-ca-1"
						}`,
						Code: http.StatusOK,
					},
					"/metadata/private/federation.json": {
						Response: `{
						 "ingressGateways": [
						   {"address": "some-actual.host", "port": 111}
						 ],
                         "publicServices": [
                           {"hostname": "some-actual.host", "ports": [{"name": "ppp", "port": 111}]}
						 ]
						}`,
						Code: http.StatusOK,
					},
				},
				"proper-hostname-2": {
					"/metadata/public/public.json": {
						Response: `{
						  "clusterUUID": "proper-uuid-2",
						  "authnKeyPub": "proper-authn-2",
						  "rootCA": "proper-root-ca-2"
						}`,
						Code: http.StatusOK,
					},
					"/metadata/private/federation.json": {
						Response: `{
						 "ingressGateways": [
						   {"address": "some-actual.host-1", "port": 111},
						   {"address": "some-actual.host-2", "port": 111}
						 ],
						 "publicServices": [
						   {"hostname": "some-actual.host-1", "ports": [{"name": "ppp", "port": 111}]},
						   {"hostname": "some-actual.host-2", "ports": [{"name": "ppp", "port": 111}]},
						   {"hostname": "some-actual.host-3", "ports": [{"name": "ppp", "port": 222}]},
                           {"hostname": "some-actual.host-4", "ports": [{"name": "https-ppp", "port": 222}]},
                           {"hostname": "some-actual.host-5", "ports": [{"name": "grpc-ppp", "port": 222}]},
                           {"hostname": "some-actual.host-6", "ports": [{"name": "http-ppp", "port": 222}]}
						 ]
						}`,
						Code: http.StatusOK,
					},
				},
			}
			dependency.TestDC.HTTPClient.DoMock.
				Set(func(req *http.Request) (*http.Response, error) {
					host := strings.Split(req.Host, ":")[0]
					uri := req.URL.Path
					mockResponse := respMap[host][uri]
					reqTokenString := req.Header.Get("Authorization")
					if strings.HasPrefix(reqTokenString, "Bearer ") {
						bearerTokens[host] = strings.TrimPrefix(reqTokenString, "Bearer ")
					}
					return &http.Response{
						Header:     map[string][]string{"Content-Type": {"application/json"}},
						StatusCode: mockResponse.Code,
						Body:       io.NopCloser(bytes.NewBufferString(mockResponse.Response)),
					}, nil
				})
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"Cluster name: my.cluster connected successfully, published services: 2\""))

			var fed0Conds []discoveryConditionRow
			Expect(json.Unmarshal([]byte(f.KubernetesGlobalResource("IstioFederation", "proper-federation-0").Field("status.conditions").String()), &fed0Conds)).To(Succeed())
			Expect(fed0Conds).To(HaveLen(2))
			fed0 := discoveryConditionsByType(fed0Conds)
			Expect(fed0["PublicMetadataExchangeReady"].Status).To(Equal("True"))
			Expect(fed0["PrivateMetadataExchangeReady"].Status).To(Equal("True"))
			tFed0PubProbe, err := time.Parse(time.RFC3339, fed0["PublicMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tFed0PubTrans, err := time.Parse(time.RFC3339, fed0["PublicMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tFed0PrivProbe, err := time.Parse(time.RFC3339, fed0["PrivateMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tFed0PrivTrans, err := time.Parse(time.RFC3339, fed0["PrivateMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			Expect(tFed0PubProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tFed0PubTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tFed0PrivProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tFed0PrivTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))

			var fed1Conds []discoveryConditionRow
			Expect(json.Unmarshal([]byte(f.KubernetesGlobalResource("IstioFederation", "proper-federation-1").Field("status.conditions").String()), &fed1Conds)).To(Succeed())
			Expect(fed1Conds).To(HaveLen(2))
			fed1 := discoveryConditionsByType(fed1Conds)
			Expect(fed1["PublicMetadataExchangeReady"].Status).To(Equal("True"))
			Expect(fed1["PrivateMetadataExchangeReady"].Status).To(Equal("True"))
			tFed1PubProbe, err := time.Parse(time.RFC3339, fed1["PublicMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tFed1PubTrans, err := time.Parse(time.RFC3339, fed1["PublicMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tFed1PrivProbe, err := time.Parse(time.RFC3339, fed1["PrivateMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tFed1PrivTrans, err := time.Parse(time.RFC3339, fed1["PrivateMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			Expect(tFed1PubProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tFed1PubTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tFed1PrivProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tFed1PrivTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))

			var fed2Conds []discoveryConditionRow
			Expect(json.Unmarshal([]byte(f.KubernetesGlobalResource("IstioFederation", "proper-federation-2").Field("status.conditions").String()), &fed2Conds)).To(Succeed())
			Expect(fed2Conds).To(HaveLen(2))
			fed2 := discoveryConditionsByType(fed2Conds)
			Expect(fed2["PublicMetadataExchangeReady"].Status).To(Equal("True"))
			Expect(fed2["PrivateMetadataExchangeReady"].Status).To(Equal("True"))
			tFed2PubProbe, err := time.Parse(time.RFC3339, fed2["PublicMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tFed2PubTrans, err := time.Parse(time.RFC3339, fed2["PublicMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tFed2PrivProbe, err := time.Parse(time.RFC3339, fed2["PrivateMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tFed2PrivTrans, err := time.Parse(time.RFC3339, fed2["PrivateMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			Expect(tFed2PubProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tFed2PubTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tFed2PrivProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tFed2PrivTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))

			Expect(f.KubernetesGlobalResource("IstioFederation", "proper-federation-0").Field("status.metadataCache.public").String()).To(MatchJSON(`
				{
					"clusterUUID": "proper-uuid-0",
					"authnKeyPub": "proper-authn-0",
					"rootCA": "proper-root-ca-0"
				}
	`))
			Expect(f.KubernetesGlobalResource("IstioFederation", "proper-federation-1").Field("status.metadataCache.public").String()).To(MatchJSON(`
				{
					"clusterUUID": "proper-uuid-1",
					"authnKeyPub": "proper-authn-1",
					"rootCA": "proper-root-ca-1"
				}
	`))
			Expect(f.KubernetesGlobalResource("IstioFederation", "proper-federation-2").Field("status.metadataCache.public").String()).To(MatchJSON(`
				{
					"clusterUUID": "proper-uuid-2",
					"authnKeyPub": "proper-authn-2",
					"rootCA": "proper-root-ca-2"
				}
	`))

			Expect(f.KubernetesGlobalResource("IstioFederation", "proper-federation-0").Field("status.metadataCache.private.ingressGateways").String()).To(MatchJSON(`
	           [
	             {"address": "a.b.c", "port": 123},
	             {"address": "1.2.3.4", "port": 234}
	           ]
	`))
			Expect(f.KubernetesGlobalResource("IstioFederation", "proper-federation-1").Field("status.metadataCache.private.ingressGateways").String()).To(MatchJSON(`
	           [
	             {"address": "some-actual.host", "port": 111}
	           ]
	`))
			Expect(f.KubernetesGlobalResource("IstioFederation", "proper-federation-2").Field("status.metadataCache.private.ingressGateways").String()).To(MatchJSON(`
	           [
	             {"address": "some-actual.host-1", "port": 111},
	             {"address": "some-actual.host-2", "port": 111}
	           ]
	`))

			Expect(f.KubernetesGlobalResource("IstioFederation", "proper-federation-0").Field("status.metadataCache.private.publicServices").String()).To(MatchJSON(`
            [
              {"hostname": "a.b.c", "ports": [{"name": "ppp", "port": 123, "protocol": "TCP"}]},
              {"hostname": "1.2.3.4", "ports": [{"name": "ppp", "port": 234, "protocol": "TCP"}]}
            ]
`))
			Expect(f.KubernetesGlobalResource("IstioFederation", "proper-federation-1").Field("status.metadataCache.private.publicServices").String()).To(MatchJSON(`
            [
              {"hostname": "some-actual.host", "ports": [{"name": "ppp", "port": 111, "protocol": "TCP"}]}
            ]
`))
			Expect(f.KubernetesGlobalResource("IstioFederation", "proper-federation-2").Field("status.metadataCache.private.publicServices").String()).To(MatchJSON(`
            [
              {"hostname": "some-actual.host-1", "ports": [{"name": "ppp", "port": 111, "protocol": "TCP"}]},
              {"hostname": "some-actual.host-2", "ports": [{"name": "ppp", "port": 111, "protocol": "TCP"}]},
              {"hostname": "some-actual.host-3", "ports": [{"name": "ppp", "port": 222, "protocol": "TCP"}]},
              {"hostname": "some-actual.host-4", "ports": [{"name": "https-ppp", "port": 222, "protocol": "TLS"}]},
              {"hostname": "some-actual.host-5", "ports": [{"name": "grpc-ppp", "port": 222, "protocol": "HTTP2"}]},
              {"hostname": "some-actual.host-6", "ports": [{"name": "http-ppp", "port": 222, "protocol": "HTTP"}]}
            ]
`))

			tokenPF0, errpf0p := jose.ParseSigned(bearerTokens["proper-hostname-0"])
			Expect(errpf0p).ShouldNot(HaveOccurred())
			tokenPF1, errpf1p := jose.ParseSigned(bearerTokens["proper-hostname-1"])
			Expect(errpf1p).ShouldNot(HaveOccurred())
			tokenPF2, errpf2p := jose.ParseSigned(bearerTokens["proper-hostname-2"])
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

			Expect(tokenPF0Payload.Aud).To(Equal("proper-uuid-0"))
			Expect(tokenPF1Payload.Aud).To(Equal("proper-uuid-1"))
			Expect(tokenPF2Payload.Aud).To(Equal("proper-uuid-2"))

			Expect(tokenPF0Payload.Scope).To(Equal("private-federation"))
			Expect(tokenPF1Payload.Scope).To(Equal("private-federation"))
			Expect(tokenPF2Payload.Scope).To(Equal("private-federation"))

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

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(7))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  federationMetricsGroup,
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   federationMetricName,
				Group:  federationMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"federation_name": "proper-federation-0",
					"endpoint":        "https://proper-hostname-0/metadata/public/public.json",
				},
			}))
			Expect(m[2]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   federationMetricName,
				Group:  federationMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"federation_name": "proper-federation-0",
					"endpoint":        "https://proper-hostname-0/metadata/private/federation.json",
				},
			}))
			Expect(m[3]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   federationMetricName,
				Group:  federationMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"federation_name": "proper-federation-1",
					"endpoint":        "https://proper-hostname-1/metadata/public/public.json",
				},
			}))
			Expect(m[4]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   federationMetricName,
				Group:  federationMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"federation_name": "proper-federation-1",
					"endpoint":        "https://proper-hostname-1/metadata/private/federation.json",
				},
			}))
			Expect(m[5]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   federationMetricName,
				Group:  federationMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"federation_name": "proper-federation-2",
					"endpoint":        "https://proper-hostname-2/metadata/public/public.json",
				},
			}))
			Expect(m[6]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   federationMetricName,
				Group:  federationMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"federation_name": "proper-federation-2",
					"endpoint":        "https://proper-hostname-2/metadata/private/federation.json",
				},
			}))
		})
	})

	Context("Improper federation", func() {
		BeforeEach(func() {
			f.ValuesSet(`istio.federation.enabled`, true)
			f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
 name: local-federation
spec:
 trustDomain: "my.cluster" # local clusterDomain
 metadataEndpoint: "https://local-hostname/metadata/"
status: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
 name: public-internal-error
spec:
 trustDomain: "pubie"
 metadataEndpoint: "https://public-internal-error/metadata/"
status: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
 name: public-bad-json
spec:
 trustDomain: "pubbj"
 metadataEndpoint: "https://public-bad-json/metadata/"
status: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
 name: public-wrong-format
spec:
 trustDomain: "pubwf"
 metadataEndpoint: "https://public-wrong-format/metadata/"
status: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
 name: private-internal-error
spec:
 trustDomain: "privie"
 metadataEndpoint: "https://private-internal-error/metadata/"
status: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
 name: private-bad-json
spec:
 trustDomain: "privbj"
 metadataEndpoint: "https://private-bad-json/metadata/"
status: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
 name: private-wrong-format
spec:
 trustDomain: "privwf"
 metadataEndpoint: "https://private-wrong-format/metadata/"
status: {}
`)
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))

			//             host       url    response
			respMap := map[string]map[string]HTTPMockResponse{
				"public-internal-error": {
					"/metadata/public/public.json": {
						Response: `some-error`,
						Code:     http.StatusInternalServerError,
					},
				},
				"public-bad-json": {
					"/metadata/public/public.json": {
						Response: `{"zzz`,
						Code:     http.StatusOK,
					},
				},
				"public-wrong-format": {
					"/metadata/public/public.json": {
						Response: `{"wrong": "format"}`,
						Code:     http.StatusOK,
					},
				},
				"private-internal-error": {
					"/metadata/public/public.json": {
						Response: `{
						  "clusterUUID": "proper-uuid-ie",
						  "authnKeyPub": "proper-authn-ie",
						  "rootCA": "proper-root-ca-ie"
						}`,
						Code: http.StatusOK,
					},
					"/metadata/private/federation.json": {
						Response: `some-error`,
						Code:     http.StatusInternalServerError,
					},
				},
				"private-bad-json": {
					"/metadata/public/public.json": {
						Response: `{
						  "clusterUUID": "proper-uuid-bj",
						  "authnKeyPub": "proper-authn-bj",
						  "rootCA": "proper-root-ca-bj"
						}`,
						Code: http.StatusOK,
					},
					"/metadata/private/federation.json": {
						Response: `{"zzz`,
						Code:     http.StatusOK,
					},
				},
				"private-wrong-format": {
					"/metadata/public/public.json": {
						Response: `{
						  "clusterUUID": "proper-uuid-wf",
						  "authnKeyPub": "proper-authn-wf",
						  "rootCA": "proper-root-ca-wf"
						}`,
						Code: http.StatusOK,
					},
					"/metadata/private/federation.json": {
						Response: `{"wrong": "format"}`,
						Code:     http.StatusOK,
					},
				},
			}
			dependency.TestDC.HTTPClient.DoMock.
				Set(func(req *http.Request) (*http.Response, error) {
					host := strings.Split(req.Host, ":")[0]
					uri := req.URL.Path
					mockResponse := respMap[host][uri]
					return &http.Response{
						Header:     map[string][]string{"Content-Type": {"application/json"}},
						StatusCode: mockResponse.Code,
						Body:       io.NopCloser(bytes.NewBufferString(mockResponse.Response)),
					}, nil
				})

			f.RunHook()
		})

		It("Hook must execute successfully with proper warnings", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(string(f.LoggerOutput.Contents())).To(Not(ContainSubstring("local-federation")))

			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"cannot fetch private metadata endpoint for IstioFederation\",\"endpoint\":\"https://private-internal-error/metadata/private/federation.json\",\"http_code\":500"))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"cannot unmarshal private metadata endpoint for IstioFederation\",\"endpoint\":\"https://private-bad-json/metadata/private/federation.json\",\"error\":\"unexpected end of JSON input\",\"name\":\"private-bad-json\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"bad public metadata format in endpoint for IstioFederation\",\"endpoint\":\"https://public-wrong-format/metadata/public/public.json\",\"name\":\"public-wrong-format\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"cannot fetch private metadata endpoint for IstioFederation\",\"endpoint\":\"https://private-internal-error/metadata/private/federation.json\",\"http_code\":500,\"name\":\"private-internal-error\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"cannot unmarshal private metadata endpoint for IstioFederation\",\"endpoint\":\"https://private-bad-json/metadata/private/federation.json\",\"error\":\"unexpected end of JSON input\",\"name\":\"private-bad-json\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"bad private metadata format in endpoint for IstioFederation\",\"endpoint\":\"https://private-wrong-format/metadata/private/federation.json\",\"name\":\"private-wrong-format\""))

			Expect(f.KubernetesGlobalResource("IstioFederation", "local-federation").Field("status").String()).To(MatchJSON("{}"))

			var publicInternalErrorConds []discoveryConditionRow
			Expect(json.Unmarshal([]byte(f.KubernetesGlobalResource("IstioFederation", "public-internal-error").Field("status.conditions").String()), &publicInternalErrorConds)).To(Succeed())
			Expect(publicInternalErrorConds).To(HaveLen(2))
			pie := discoveryConditionsByType(publicInternalErrorConds)
			Expect(pie["PublicMetadataExchangeReady"].Status).To(Equal("False"))
			Expect(pie["PublicMetadataExchangeReady"].Reason).To(Equal("NonOKResponse"))
			Expect(pie["PublicMetadataExchangeReady"].Message).To(ContainSubstring("HTTP status 500"))
			Expect(pie["PrivateMetadataExchangeReady"].Status).To(Equal("Unknown"))
			Expect(pie["PrivateMetadataExchangeReady"].Reason).To(Equal("AwaitingPublic"))
			tPiePubProbe, err := time.Parse(time.RFC3339, pie["PublicMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tPiePubTrans, err := time.Parse(time.RFC3339, pie["PublicMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tPiePrivProbe, err := time.Parse(time.RFC3339, pie["PrivateMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tPiePrivTrans, err := time.Parse(time.RFC3339, pie["PrivateMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			Expect(tPiePubProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tPiePubTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tPiePrivProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tPiePrivTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))

			var publicBadJSONConds []discoveryConditionRow
			Expect(json.Unmarshal([]byte(f.KubernetesGlobalResource("IstioFederation", "public-bad-json").Field("status.conditions").String()), &publicBadJSONConds)).To(Succeed())
			Expect(publicBadJSONConds).To(HaveLen(2))
			pbj := discoveryConditionsByType(publicBadJSONConds)
			Expect(pbj["PublicMetadataExchangeReady"].Status).To(Equal("False"))
			Expect(pbj["PublicMetadataExchangeReady"].Reason).To(Equal("InvalidJSON"))
			Expect(pbj["PrivateMetadataExchangeReady"].Status).To(Equal("Unknown"))
			tPbjPubProbe, err := time.Parse(time.RFC3339, pbj["PublicMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tPbjPubTrans, err := time.Parse(time.RFC3339, pbj["PublicMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tPbjPrivProbe, err := time.Parse(time.RFC3339, pbj["PrivateMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tPbjPrivTrans, err := time.Parse(time.RFC3339, pbj["PrivateMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			Expect(tPbjPubProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tPbjPubTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tPbjPrivProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tPbjPrivTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))

			var publicWrongFormatConds []discoveryConditionRow
			Expect(json.Unmarshal([]byte(f.KubernetesGlobalResource("IstioFederation", "public-wrong-format").Field("status.conditions").String()), &publicWrongFormatConds)).To(Succeed())
			Expect(publicWrongFormatConds).To(HaveLen(2))
			pwf := discoveryConditionsByType(publicWrongFormatConds)
			Expect(pwf["PublicMetadataExchangeReady"].Status).To(Equal("False"))
			Expect(pwf["PublicMetadataExchangeReady"].Reason).To(Equal("InvalidPublicMetadata"))
			Expect(pwf["PrivateMetadataExchangeReady"].Status).To(Equal("Unknown"))
			tPwfPubProbe, err := time.Parse(time.RFC3339, pwf["PublicMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tPwfPubTrans, err := time.Parse(time.RFC3339, pwf["PublicMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tPwfPrivProbe, err := time.Parse(time.RFC3339, pwf["PrivateMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tPwfPrivTrans, err := time.Parse(time.RFC3339, pwf["PrivateMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			Expect(tPwfPubProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tPwfPubTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tPwfPrivProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tPwfPrivTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))

			Expect(f.KubernetesGlobalResource("IstioFederation", "private-internal-error").Field("status.metadataCache.public").String()).To(MatchJSON(`{
						  "clusterUUID": "proper-uuid-ie",
						  "authnKeyPub": "proper-authn-ie",
						  "rootCA": "proper-root-ca-ie"
			}`))
			Expect(f.KubernetesGlobalResource("IstioFederation", "private-bad-json").Field("status.metadataCache.public").String()).To(MatchJSON(`{
						  "clusterUUID": "proper-uuid-bj",
						  "authnKeyPub": "proper-authn-bj",
						  "rootCA": "proper-root-ca-bj"
			}`))
			Expect(f.KubernetesGlobalResource("IstioFederation", "private-wrong-format").Field("status.metadataCache.public").String()).To(MatchJSON(`{
						  "clusterUUID": "proper-uuid-wf",
						  "authnKeyPub": "proper-authn-wf",
						  "rootCA": "proper-root-ca-wf"
			}`))

			Expect(f.KubernetesGlobalResource("IstioFederation", "private-internal-error").Field("status.metadataCache.private").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("IstioFederation", "private-bad-json").Field("status.metadataCache.private").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("IstioFederation", "private-wrong-format").Field("status.metadataCache.private").Exists()).To(BeFalse())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(10))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  federationMetricsGroup,
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   federationMetricName,
				Group:  federationMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"federation_name": "private-bad-json",
					"endpoint":        "https://private-bad-json/metadata/public/public.json",
				},
			}))
			Expect(m[2]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   federationMetricName,
				Group:  federationMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"federation_name": "private-bad-json",
					"endpoint":        "https://private-bad-json/metadata/private/federation.json",
				},
			}))
			Expect(m[3]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   federationMetricName,
				Group:  federationMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"federation_name": "private-internal-error",
					"endpoint":        "https://private-internal-error/metadata/public/public.json",
				},
			}))
			Expect(m[4]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   federationMetricName,
				Group:  federationMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"federation_name": "private-internal-error",
					"endpoint":        "https://private-internal-error/metadata/private/federation.json",
				},
			}))
			Expect(m[5]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   federationMetricName,
				Group:  federationMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"federation_name": "private-wrong-format",
					"endpoint":        "https://private-wrong-format/metadata/public/public.json",
				},
			}))
			Expect(m[6]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   federationMetricName,
				Group:  federationMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"federation_name": "private-wrong-format",
					"endpoint":        "https://private-wrong-format/metadata/private/federation.json",
				},
			}))
			Expect(m[7]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   federationMetricName,
				Group:  federationMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"federation_name": "public-bad-json",
					"endpoint":        "https://public-bad-json/metadata/public/public.json",
				},
			}))
			Expect(m[8]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   federationMetricName,
				Group:  federationMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"federation_name": "public-internal-error",
					"endpoint":        "https://public-internal-error/metadata/public/public.json",
				},
			}))
			Expect(m[9]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   federationMetricName,
				Group:  federationMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"federation_name": "public-wrong-format",
					"endpoint":        "https://public-wrong-format/metadata/public/public.json",
				},
			}))
		})
	})
})

// discoveryConditionRow mirrors status.conditions[] items.
type discoveryConditionRow struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	Reason             string `json:"reason"`
	Message            string `json:"message"`
	LastProbeTime      string `json:"lastProbeTime"`
	LastTransitionTime string `json:"lastTransitionTime"`
}

func discoveryConditionsByType(rows []discoveryConditionRow) map[string]discoveryConditionRow {
	out := make(map[string]discoveryConditionRow, len(rows))
	for _, r := range rows {
		out[r.Type] = r
	}
	return out
}
