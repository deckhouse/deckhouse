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
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/square/go-jose/v3"
	"k8s.io/utils/ptr"

	eeCrd "github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/ee/lib/crd"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: multicluster_discovery ::", func() {
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

	Context("Empty cluster, minimal settings and multicluster is enabled", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.multicluster.enabled", true)
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

	Context("Proper multiclusters only", func() {
		var bearerTokens = map[string]string{}

		BeforeEach(func() {
			f.ValuesSet(`istio.multicluster.enabled`, true)
			f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: proper-multicluster-0
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://proper-hostname-0/metadata/"
status: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: proper-multicluster-1
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://proper-hostname-1/metadata/"
status:
  metadataCache:
    private:
      ingressGateways:
      - {"address": "some-outdated.host", "port": 111} # must be overwritten by the new data
      ambientGateways:
      - {"address": "10.0.0.9", "port": 15008} # peer no longer publishes any, must be cleared
      apiHost: some-outdatad-api.host
      networkName: some-outdated-networkname
    public:
      clusterUUID: proper-uuid-1 # pinned after first successful discovery
      rootCA: bad-root-ca
      authnKeyPub: bad-authn-key-pub
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: proper-multicluster-2
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://proper-hostname-2/metadata/"
status:
  metadataCache:
    ingressGateways:
    - {"address": "some-actual.host-1", "port": 111} # should be saved
    - {"address": "some-outdatad.host-2", "port": 111} # should be deleted
`)
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))

			apiVersionsProbeBody := `{"kind":"APIVersions","versions":[]}`
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
					"/metadata/private/multicluster.json": {
						Response: `{
						  "ingressGateways": [
							{"address": "a.b.c", "port": 123},
							{"address": "1.2.3.4", "port": 234}
						  ],
						  "ambientGateways": [
							{"address": "1.2.3.4", "port": 15008}
						  ],
                          "apiHost": "api-host-0",
                          "clusterID": "cluster-id-0",
                          "networkName": "network-name-0"
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
					"/metadata/private/multicluster.json": {
						Response: `{
						 "ingressGateways": [
						   {"address": "some-actual.host", "port": 111}
						 ],
                          "apiHost": "api-host-1",
                          "networkName": "network-name-1"
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
					"/metadata/private/multicluster.json": {
						Response: `{
						 "ingressGateways": [
						   {"address": "some-actual.host-1", "port": 111},
						   {"address": "some-actual.host-2", "port": 111}
						 ],
                          "apiHost": "api-host-2",
                          "networkName": "network-name-2"
						}`,
						Code: http.StatusOK,
					},
				},
				// Remote API readiness probe: GET https://<apiHost>/api (Bearer JWT, scope api).
				"api-host-0": {
					"/api": {Response: apiVersionsProbeBody, Code: http.StatusOK},
				},
				"api-host-1": {
					"/api": {Response: apiVersionsProbeBody, Code: http.StatusOK},
				},
				"api-host-2": {
					"/api": {Response: apiVersionsProbeBody, Code: http.StatusOK},
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
			Expect(string(f.LoggerOutput.Contents())).To(HaveLen(0))

			var mc0Conds []discoveryConditionRow
			Expect(json.Unmarshal([]byte(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-0").Field("status.conditions").String()), &mc0Conds)).To(Succeed())
			Expect(mc0Conds).To(HaveLen(3))
			mc0 := discoveryConditionsByType(mc0Conds)
			Expect(mc0["PublicMetadataExchangeReady"].Status).To(Equal("True"))
			Expect(mc0["PrivateMetadataExchangeReady"].Status).To(Equal("True"))
			Expect(mc0["RemoteAPIServerReady"].Status).To(Equal("True"))
			Expect(mc0["RemoteAPIServerReady"].Reason).To(Equal("RemoteAPIReachable"))
			tMc0PubProbe, err := time.Parse(time.RFC3339, mc0["PublicMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMc0PubTrans, err := time.Parse(time.RFC3339, mc0["PublicMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tMc0PrivProbe, err := time.Parse(time.RFC3339, mc0["PrivateMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMc0PrivTrans, err := time.Parse(time.RFC3339, mc0["PrivateMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tMc0RemProbe, err := time.Parse(time.RFC3339, mc0["RemoteAPIServerReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMc0RemTrans, err := time.Parse(time.RFC3339, mc0["RemoteAPIServerReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			Expect(tMc0PubProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMc0PubTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMc0PrivProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMc0PrivTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMc0RemProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMc0RemTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))

			var mc1Conds []discoveryConditionRow
			Expect(json.Unmarshal([]byte(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-1").Field("status.conditions").String()), &mc1Conds)).To(Succeed())
			Expect(mc1Conds).To(HaveLen(3))
			mc1 := discoveryConditionsByType(mc1Conds)
			Expect(mc1["PublicMetadataExchangeReady"].Status).To(Equal("True"))
			Expect(mc1["PrivateMetadataExchangeReady"].Status).To(Equal("True"))
			Expect(mc1["RemoteAPIServerReady"].Status).To(Equal("True"))
			Expect(mc1["RemoteAPIServerReady"].Reason).To(Equal("RemoteAPIReachable"))
			tMc1PubProbe, err := time.Parse(time.RFC3339, mc1["PublicMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMc1PubTrans, err := time.Parse(time.RFC3339, mc1["PublicMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tMc1PrivProbe, err := time.Parse(time.RFC3339, mc1["PrivateMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMc1PrivTrans, err := time.Parse(time.RFC3339, mc1["PrivateMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tMc1RemProbe, err := time.Parse(time.RFC3339, mc1["RemoteAPIServerReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMc1RemTrans, err := time.Parse(time.RFC3339, mc1["RemoteAPIServerReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			Expect(tMc1PubProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMc1PubTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMc1PrivProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMc1PrivTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMc1RemProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMc1RemTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))

			var mc2Conds []discoveryConditionRow
			Expect(json.Unmarshal([]byte(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-2").Field("status.conditions").String()), &mc2Conds)).To(Succeed())
			Expect(mc2Conds).To(HaveLen(3))
			mc2 := discoveryConditionsByType(mc2Conds)
			Expect(mc2["PublicMetadataExchangeReady"].Status).To(Equal("True"))
			Expect(mc2["PrivateMetadataExchangeReady"].Status).To(Equal("True"))
			Expect(mc2["RemoteAPIServerReady"].Status).To(Equal("True"))
			Expect(mc2["RemoteAPIServerReady"].Reason).To(Equal("RemoteAPIReachable"))
			tMc2PubProbe, err := time.Parse(time.RFC3339, mc2["PublicMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMc2PubTrans, err := time.Parse(time.RFC3339, mc2["PublicMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tMc2PrivProbe, err := time.Parse(time.RFC3339, mc2["PrivateMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMc2PrivTrans, err := time.Parse(time.RFC3339, mc2["PrivateMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tMc2RemProbe, err := time.Parse(time.RFC3339, mc2["RemoteAPIServerReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMc2RemTrans, err := time.Parse(time.RFC3339, mc2["RemoteAPIServerReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			Expect(tMc2PubProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMc2PubTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMc2PrivProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMc2PrivTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMc2RemProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMc2RemTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))

			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-0").Field("status.metadataCache.public").String()).To(MatchJSON(`
				{
					"clusterUUID": "proper-uuid-0",
					"authnKeyPub": "proper-authn-0",
					"rootCA": "proper-root-ca-0"
				}
	`))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-1").Field("status.metadataCache.public").String()).To(MatchJSON(`
				{
					"clusterUUID": "proper-uuid-1",
					"authnKeyPub": "proper-authn-1",
					"rootCA": "proper-root-ca-1"
				}
	`))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-2").Field("status.metadataCache.public").String()).To(MatchJSON(`
				{
					"clusterUUID": "proper-uuid-2",
					"authnKeyPub": "proper-authn-2",
					"rootCA": "proper-root-ca-2"
				}
	`))

			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-0").Field("status.metadataCache.private.ingressGateways").String()).To(MatchJSON(`
	           [
	             {"address": "a.b.c", "port": 123},
	             {"address": "1.2.3.4", "port": 234}
	           ]
	`))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-1").Field("status.metadataCache.private.ingressGateways").String()).To(MatchJSON(`
	           [
	             {"address": "some-actual.host", "port": 111}
	           ]
	`))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-2").Field("status.metadataCache.private.ingressGateways").String()).To(MatchJSON(`
	           [
	             {"address": "some-actual.host-1", "port": 111},
	             {"address": "some-actual.host-2", "port": 111}
	           ]
	`))

			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-0").Field("status.metadataCache.private.ambientGateways").String()).To(MatchJSON(`
	           [
	             {"address": "1.2.3.4", "port": 15008}
	           ]
	`))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-1").Field("status.metadataCache.private.ambientGateways").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-2").Field("status.metadataCache.private.ambientGateways").Exists()).To(BeFalse())

			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-0").Field("status.metadataCache.private.apiHost").String()).To(Equal("api-host-0"))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-1").Field("status.metadataCache.private.apiHost").String()).To(Equal("api-host-1"))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-2").Field("status.metadataCache.private.apiHost").String()).To(Equal("api-host-2"))

			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-0").Field("status.metadataCache.private.networkName").String()).To(Equal("network-name-0"))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-1").Field("status.metadataCache.private.networkName").String()).To(Equal("network-name-1"))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-2").Field("status.metadataCache.private.networkName").String()).To(Equal("network-name-2"))

			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-0").Field("status.metadataCache.private.clusterID").String()).To(Equal("cluster-id-0"))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-1").Field("status.metadataCache.private.clusterID").Exists()).To(BeFalse())

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

			Expect(tokenPF0Payload.Scope).To(Equal("private-multicluster"))
			Expect(tokenPF1Payload.Scope).To(Equal("private-multicluster"))
			Expect(tokenPF2Payload.Scope).To(Equal("private-multicluster"))

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
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"multicluster_name": "proper-multicluster-0",
					"endpoint":          "https://proper-hostname-0/metadata/public/public.json",
				},
			}))
			Expect(m[2]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"multicluster_name": "proper-multicluster-0",
					"endpoint":          "https://proper-hostname-0/metadata/private/multicluster.json",
				},
			}))
			Expect(m[3]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"multicluster_name": "proper-multicluster-1",
					"endpoint":          "https://proper-hostname-1/metadata/public/public.json",
				},
			}))
			Expect(m[4]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"multicluster_name": "proper-multicluster-1",
					"endpoint":          "https://proper-hostname-1/metadata/private/multicluster.json",
				},
			}))
			Expect(m[5]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"multicluster_name": "proper-multicluster-2",
					"endpoint":          "https://proper-hostname-2/metadata/public/public.json",
				},
			}))
			Expect(m[6]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"multicluster_name": "proper-multicluster-2",
					"endpoint":          "https://proper-hostname-2/metadata/private/multicluster.json",
				},
			}))
		})
	})

	Context("Improper multicluster", func() {
		BeforeEach(func() {
			f.ValuesSet(`istio.multicluster.enabled`, true)
			f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
 name: public-internal-error
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://public-internal-error/metadata/"
status: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
 name: public-bad-json
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://public-bad-json/metadata/"
status: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
 name: public-wrong-format
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://public-wrong-format/metadata/"
status: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
 name: private-internal-error
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://private-internal-error/metadata/"
status: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
 name: private-bad-json
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://private-bad-json/metadata/"
status: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
 name: private-wrong-format
spec:
  enableIngressGateway: true
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
					"/metadata/private/multicluster.json": {
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
					"/metadata/private/multicluster.json": {
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
					"/metadata/private/multicluster.json": {
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

			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"cannot fetch private metadata endpoint for IstioMulticluster\",\"endpoint\":\"https://private-internal-error/metadata/private/multicluster.json\",\"http_code\":500,\"name\":\"private-internal-error\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"bad private metadata format in endpoint for IstioMulticluster\",\"endpoint\":\"https://private-wrong-format/metadata/private/multicluster.json\",\"name\":\"private-wrong-format\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"cannot unmarshal public metadata endpoint for IstioMulticluster\",\"endpoint\":\"https://public-bad-json/metadata/public/public.json\",\"error\":\"unexpected end of JSON input\",\"name\":\"public-bad-json\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"cannot fetch public metadata endpoint for IstioMulticluster\",\"endpoint\":\"https://public-internal-error/metadata/public/public.json\",\"http_code\":500,\"name\":\"public-internal-error\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"bad public metadata format in endpoint for IstioMulticluster\",\"endpoint\":\"https://public-wrong-format/metadata/public/public.json\",\"name\":\"public-wrong-format\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"cannot unmarshal private metadata endpoint for IstioMulticluster\",\"endpoint\":\"https://private-bad-json/metadata/private/multicluster.json\",\"error\":\"unexpected end of JSON input\",\"name\":\"private-bad-json\""))

			var mcPublicInternalErrorConds []discoveryConditionRow
			Expect(json.Unmarshal([]byte(f.KubernetesGlobalResource("IstioMulticluster", "public-internal-error").Field("status.conditions").String()), &mcPublicInternalErrorConds)).To(Succeed())
			Expect(mcPublicInternalErrorConds).To(HaveLen(3))
			mcPie := discoveryConditionsByType(mcPublicInternalErrorConds)
			Expect(mcPie["PublicMetadataExchangeReady"].Status).To(Equal("False"))
			Expect(mcPie["PublicMetadataExchangeReady"].Reason).To(Equal("NonOKResponse"))
			Expect(mcPie["PrivateMetadataExchangeReady"].Status).To(Equal("Unknown"))
			Expect(mcPie["RemoteAPIServerReady"].Status).To(Equal("Unknown"))
			Expect(mcPie["RemoteAPIServerReady"].Reason).To(Equal("AwaitingPrivate"))
			tMcPiePubProbe, err := time.Parse(time.RFC3339, mcPie["PublicMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMcPiePubTrans, err := time.Parse(time.RFC3339, mcPie["PublicMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tMcPiePrivProbe, err := time.Parse(time.RFC3339, mcPie["PrivateMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMcPiePrivTrans, err := time.Parse(time.RFC3339, mcPie["PrivateMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tMcPieRemProbe, err := time.Parse(time.RFC3339, mcPie["RemoteAPIServerReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMcPieRemTrans, err := time.Parse(time.RFC3339, mcPie["RemoteAPIServerReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			Expect(tMcPiePubProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMcPiePubTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMcPiePrivProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMcPiePrivTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMcPieRemProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMcPieRemTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))

			var mcPublicBadJSONConds []discoveryConditionRow
			Expect(json.Unmarshal([]byte(f.KubernetesGlobalResource("IstioMulticluster", "public-bad-json").Field("status.conditions").String()), &mcPublicBadJSONConds)).To(Succeed())
			Expect(mcPublicBadJSONConds).To(HaveLen(3))
			mcPbj := discoveryConditionsByType(mcPublicBadJSONConds)
			Expect(mcPbj["PublicMetadataExchangeReady"].Status).To(Equal("False"))
			Expect(mcPbj["PublicMetadataExchangeReady"].Reason).To(Equal("InvalidJSON"))
			Expect(mcPbj["PrivateMetadataExchangeReady"].Status).To(Equal("Unknown"))
			Expect(mcPbj["RemoteAPIServerReady"].Status).To(Equal("Unknown"))
			tMcPbjPubProbe, err := time.Parse(time.RFC3339, mcPbj["PublicMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMcPbjPubTrans, err := time.Parse(time.RFC3339, mcPbj["PublicMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tMcPbjPrivProbe, err := time.Parse(time.RFC3339, mcPbj["PrivateMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMcPbjPrivTrans, err := time.Parse(time.RFC3339, mcPbj["PrivateMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tMcPbjRemProbe, err := time.Parse(time.RFC3339, mcPbj["RemoteAPIServerReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMcPbjRemTrans, err := time.Parse(time.RFC3339, mcPbj["RemoteAPIServerReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			Expect(tMcPbjPubProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMcPbjPubTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMcPbjPrivProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMcPbjPrivTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMcPbjRemProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMcPbjRemTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))

			var mcPublicWrongFormatConds []discoveryConditionRow
			Expect(json.Unmarshal([]byte(f.KubernetesGlobalResource("IstioMulticluster", "public-wrong-format").Field("status.conditions").String()), &mcPublicWrongFormatConds)).To(Succeed())
			Expect(mcPublicWrongFormatConds).To(HaveLen(3))
			mcPwf := discoveryConditionsByType(mcPublicWrongFormatConds)
			Expect(mcPwf["PublicMetadataExchangeReady"].Status).To(Equal("False"))
			Expect(mcPwf["PublicMetadataExchangeReady"].Reason).To(Equal("InvalidPublicMetadata"))
			Expect(mcPwf["PrivateMetadataExchangeReady"].Status).To(Equal("Unknown"))
			Expect(mcPwf["RemoteAPIServerReady"].Status).To(Equal("Unknown"))
			tMcPwfPubProbe, err := time.Parse(time.RFC3339, mcPwf["PublicMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMcPwfPubTrans, err := time.Parse(time.RFC3339, mcPwf["PublicMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tMcPwfPrivProbe, err := time.Parse(time.RFC3339, mcPwf["PrivateMetadataExchangeReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMcPwfPrivTrans, err := time.Parse(time.RFC3339, mcPwf["PrivateMetadataExchangeReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			tMcPwfRemProbe, err := time.Parse(time.RFC3339, mcPwf["RemoteAPIServerReady"].LastProbeTime)
			Expect(err).NotTo(HaveOccurred())
			tMcPwfRemTrans, err := time.Parse(time.RFC3339, mcPwf["RemoteAPIServerReady"].LastTransitionTime)
			Expect(err).NotTo(HaveOccurred())
			Expect(tMcPwfPubProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMcPwfPubTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMcPwfPrivProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMcPwfPrivTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMcPwfRemProbe).To(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tMcPwfRemTrans).To(BeTemporally("~", time.Now().UTC(), time.Minute))

			Expect(f.KubernetesGlobalResource("IstioMulticluster", "private-internal-error").Field("status.metadataCache.public").String()).To(MatchJSON(`{
						  "clusterUUID": "proper-uuid-ie",
						  "authnKeyPub": "proper-authn-ie",
						  "rootCA": "proper-root-ca-ie"
			}`))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "private-bad-json").Field("status.metadataCache.public").String()).To(MatchJSON(`{
						  "clusterUUID": "proper-uuid-bj",
						  "authnKeyPub": "proper-authn-bj",
						  "rootCA": "proper-root-ca-bj"
			}`))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "private-wrong-format").Field("status.metadataCache.public").String()).To(MatchJSON(`{
						  "clusterUUID": "proper-uuid-wf",
						  "authnKeyPub": "proper-authn-wf",
						  "rootCA": "proper-root-ca-wf"
			}`))

			Expect(f.KubernetesGlobalResource("IstioMulticluster", "private-internal-error").Field("status.metadataCache.private").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "private-bad-json").Field("status.metadataCache.private").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "private-wrong-format").Field("status.metadataCache.private").Exists()).To(BeFalse())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(10))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"multicluster_name": "private-bad-json",
					"endpoint":          "https://private-bad-json/metadata/public/public.json",
				},
			}))
			Expect(m[2]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"multicluster_name": "private-bad-json",
					"endpoint":          "https://private-bad-json/metadata/private/multicluster.json",
				},
			}))
			Expect(m[3]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"multicluster_name": "private-internal-error",
					"endpoint":          "https://private-internal-error/metadata/public/public.json",
				},
			}))
			Expect(m[4]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"multicluster_name": "private-internal-error",
					"endpoint":          "https://private-internal-error/metadata/private/multicluster.json",
				},
			}))
			Expect(m[5]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
				Labels: map[string]string{
					"multicluster_name": "private-wrong-format",
					"endpoint":          "https://private-wrong-format/metadata/public/public.json",
				},
			}))
			Expect(m[6]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"multicluster_name": "private-wrong-format",
					"endpoint":          "https://private-wrong-format/metadata/private/multicluster.json",
				},
			}))
			Expect(m[7]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"multicluster_name": "public-bad-json",
					"endpoint":          "https://public-bad-json/metadata/public/public.json",
				},
			}))
			Expect(m[8]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"multicluster_name": "public-internal-error",
					"endpoint":          "https://public-internal-error/metadata/public/public.json",
				},
			}))
			Expect(m[9]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"multicluster_name": "public-wrong-format",
					"endpoint":          "https://public-wrong-format/metadata/public/public.json",
				},
			}))
		})
	})

	Context("Pinned remote cluster UUID", func() {
		privateEndpointRequested := false

		BeforeEach(func() {
			privateEndpointRequested = false
			f.ValuesSet("istio.multicluster.enabled", true)
			f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: uuid-mismatch
spec:
  metadataEndpoint: https://uuid-mismatch/metadata/
status:
  metadataCache:
    public:
      clusterUUID: pinned-uuid
      authnKeyPub: pinned-authn-key
      rootCA: pinned-root-ca
`)
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			dependency.TestDC.HTTPClient.DoMock.
				Set(func(req *http.Request) (*http.Response, error) {
					if req.URL.Path == "/metadata/private/multicluster.json" {
						privateEndpointRequested = true
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body: io.NopCloser(bytes.NewBufferString(`{
							"clusterUUID": "unexpected-uuid",
							"authnKeyPub": "remote-authn-key",
							"rootCA": "remote-root-ca"
						}`)),
					}, nil
				})
			f.RunHook()
		})

		It("does not issue a JWT or overwrite pinned public metadata when the UUID changes", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(privateEndpointRequested).To(BeFalse())
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "uuid-mismatch").Field("status.metadataCache.public.clusterUUID").String()).To(Equal("pinned-uuid"))

			var conditions []discoveryConditionRow
			Expect(json.Unmarshal([]byte(f.KubernetesGlobalResource("IstioMulticluster", "uuid-mismatch").Field("status.conditions").String()), &conditions)).To(Succeed())
			Expect(discoveryConditionsByType(conditions)["PublicMetadataExchangeReady"].Reason).To(Equal("ClusterUUIDMismatch"))
			Expect(discoveryConditionsByType(conditions)["PrivateMetadataExchangeReady"].Status).To(Equal("Unknown"))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(ContainElement(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterUUIDMismatchMetricName,
				Group:  multiclusterMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"multicluster_name": "uuid-mismatch",
					"endpoint":          "https://uuid-mismatch/metadata/public/public.json",
					"pinned_uuid":       "pinned-uuid",
					"actual_uuid":       "unexpected-uuid",
				},
			})))
		})
	})

	// Everything a peer publishes ends up in a rendered object: networkName as a label
	// value on an istio-remote Gateway and as a key in meshNetworks, ambient addresses as
	// spec.addresses whose CRD enforces format: ipv4|ipv6. One bad value applied would fail
	// the release and take the module down in this cluster, so it is caught on the way in.
	Context("Peer metadata that would not survive being rendered", func() {
		BeforeEach(func() {
			f.ValuesSet(`istio.multicluster.enabled`, true)
			f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: bad-network-name
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://bad-network-name/metadata/"
status: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: unusable-ambient
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://unusable-ambient/metadata/"
status:
  metadataCache:
    private:
      ambientGateways:
      - {"address": "10.0.0.9", "port": 15008} # cached from when the peer published a usable one
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: mixed-ambient
spec:
  enableIngressGateway: true
  metadataEndpoint: "https://mixed-ambient/metadata/"
status: {}
`)
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))

			apiVersionsProbeBody := `{"kind":"APIVersions","versions":[]}`
			publicMetadata := func(suffix string) string {
				return `{
				  "clusterUUID": "proper-uuid-` + suffix + `",
				  "authnKeyPub": "proper-authn-` + suffix + `",
				  "rootCA": "proper-root-ca-` + suffix + `"
				}`
			}

			respMap := map[string]map[string]HTTPMockResponse{
				"bad-network-name": {
					"/metadata/public/public.json": {Response: publicMetadata("bnn"), Code: http.StatusOK},
					"/metadata/private/multicluster.json": {
						// 70 characters, over the 63 a label value allows.
						Response: `{
						  "ingressGateways": [{"address": "1.2.3.4", "port": 111}],
						  "ambientGateways": [{"address": "10.0.0.1", "port": 15008}],
						  "apiHost": "api-host-bnn",
						  "networkName": "network-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						}`,
						Code: http.StatusOK,
					},
				},
				"unusable-ambient": {
					"/metadata/public/public.json": {Response: publicMetadata("ua"), Code: http.StatusOK},
					"/metadata/private/multicluster.json": {
						Response: `{
						  "ingressGateways": [{"address": "1.2.3.4", "port": 111}],
						  "ambientGateways": [
						    {"address": "lb.example.com", "port": 15008},
						    {"address": "999.999.999.999", "port": 15008},
						    {"address": "10.0.0.1", "port": 0}
						  ],
						  "apiHost": "api-host-ua",
						  "networkName": "network-name-ua"
						}`,
						Code: http.StatusOK,
					},
				},
				"mixed-ambient": {
					"/metadata/public/public.json": {Response: publicMetadata("ma"), Code: http.StatusOK},
					"/metadata/private/multicluster.json": {
						Response: `{
						  "ingressGateways": [{"address": "1.2.3.4", "port": 111}],
						  "ambientGateways": [
						    {"address": "not-an-address", "port": 15008},
						    {"address": "::ffff:10.0.0.2", "port": 15008}
						  ],
						  "apiHost": "api-host-ma",
						  "networkName": "network-name-ma"
						}`,
						Code: http.StatusOK,
					},
				},
				"api-host-bnn": {"/api": {Response: apiVersionsProbeBody, Code: http.StatusOK}},
				"api-host-ua":  {"/api": {Response: apiVersionsProbeBody, Code: http.StatusOK}},
				"api-host-ma":  {"/api": {Response: apiVersionsProbeBody, Code: http.StatusOK}},
			}
			dependency.TestDC.HTTPClient.DoMock.
				Set(func(req *http.Request) (*http.Response, error) {
					host := strings.Split(req.Host, ":")[0]
					mockResponse := respMap[host][req.URL.Path]
					return &http.Response{
						Header:     map[string][]string{"Content-Type": {"application/json"}},
						StatusCode: mockResponse.Code,
						Body:       io.NopCloser(bytes.NewBufferString(mockResponse.Response)),
					}, nil
				})

			f.RunHook()
		})

		// A networkName too long to be a label value costs the peer its ambient half and
		// nothing else. Sidecar multicluster only ever puts networkName in meshNetworks,
		// where the label syntax does not apply, so rejecting the peer over it would break
		// a path that was working.
		It("Keeps a peer whose networkName is not a valid label value, minus its ambient half", func() {
			Expect(f).To(ExecuteSuccessfully())

			mc := f.KubernetesGlobalResource("IstioMulticluster", "bad-network-name")
			Expect(mc.Field("status.metadataCache.private.ingressGateways").String()).To(MatchJSON(`
			  [{"address": "1.2.3.4", "port": 111}]
			`))
			Expect(mc.Field("status.metadataCache.private.networkName").String()).To(Equal(
				"network-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
			Expect(mc.Field("status.metadataCache.private.ambientGateways").Exists()).To(BeFalse())

			var conditions []discoveryConditionRow
			Expect(json.Unmarshal([]byte(mc.Field("status.conditions").String()), &conditions)).To(Succeed())
			Expect(discoveryConditionsByType(conditions)["PrivateMetadataExchangeReady"].Status).To(Equal("True"))
		})

		// An unusable ambient address costs the peer only its ambient half too: sidecar
		// multicluster does not go through this gateway.
		It("Keeps a peer whose ambient addresses are all unusable, minus its ambient half", func() {
			Expect(f).To(ExecuteSuccessfully())

			mc := f.KubernetesGlobalResource("IstioMulticluster", "unusable-ambient")
			Expect(mc.Field("status.metadataCache.private.ingressGateways").String()).To(MatchJSON(`
			  [{"address": "1.2.3.4", "port": 111}]
			`))
			// Including the one cached from before, which must not outlive the peer's
			// ability to serve it.
			Expect(mc.Field("status.metadataCache.private.ambientGateways").Exists()).To(BeFalse())

			var conditions []discoveryConditionRow
			Expect(json.Unmarshal([]byte(mc.Field("status.conditions").String()), &conditions)).To(Succeed())
			Expect(discoveryConditionsByType(conditions)["PrivateMetadataExchangeReady"].Status).To(Equal("True"))

			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("dropping ambient gateway endpoints the ambient data plane cannot use"))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("lb.example.com:15008, 999.999.999.999:15008, 10.0.0.1:0"))
		})

		It("Keeps the usable addresses of a peer that published both, canonicalised", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.KubernetesGlobalResource("IstioMulticluster", "mixed-ambient").
				Field("status.metadataCache.private.ambientGateways").String()).To(MatchJSON(`
			  [{"address": "10.0.0.2", "port": 15008}]
			`))
		})
	})
})

func TestSanitizeAmbientGateways(t *testing.T) {
	gw := func(address string, port uint) eeCrd.MulticlusterIngressGateways {
		return eeCrd.MulticlusterIngressGateways{Address: address, Port: port}
	}
	list := func(gws ...eeCrd.MulticlusterIngressGateways) *[]eeCrd.MulticlusterIngressGateways {
		return &gws
	}

	cases := []struct {
		name        string
		networkName string
		in          *[]eeCrd.MulticlusterIngressGateways
		wantKept    *[]eeCrd.MulticlusterIngressGateways
		wantDropped []string
	}{
		{
			name: "nothing published",
			in:   nil,
		},
		{
			name:     "IPv4 and IPv6 literals are kept",
			in:       list(gw("10.0.0.1", 15008), gw("2001:db8::1", 15008)),
			wantKept: list(gw("10.0.0.1", 15008), gw("2001:db8::1", 15008)),
		},
		{
			// The ambient path writes the address straight into a Workload for ztunnel and
			// never resolves it, so a DNS name is not a lesser address but a wrong one.
			name:        "a DNS name is dropped",
			in:          list(gw("lb.example.com", 15008)),
			wantDropped: []string{"lb.example.com:15008"},
		},
		{
			// Address-shaped but not an address: the octets are out of range. This is the
			// case a regex in the template got wrong.
			name:        "an out-of-range IPv4 is dropped",
			in:          list(gw("999.999.999.999", 15008)),
			wantDropped: []string{"999.999.999.999:15008"},
		},
		{
			// Colons alone made it past the template's IPv6 matcher, and would then be
			// rejected by the Gateway CRD's format: ipv6.
			name:        "a colon-shaped non-address is dropped",
			in:          list(gw("::::", 15008)),
			wantDropped: []string{"[::::]:15008"},
		},
		{
			name:        "port 0 is not a listener port",
			in:          list(gw("10.0.0.1", 0)),
			wantDropped: []string{"10.0.0.1:0"},
		},
		{
			name:        "a port above the range is dropped",
			in:          list(gw("10.0.0.1", 70000)),
			wantDropped: []string{"10.0.0.1:70000"},
		},
		{
			// Canonicalised, so the value matches what the Gateway CRD accepts and so the
			// rendered object does not churn on an equivalent spelling.
			name:     "an IPv4-mapped address is written as plain IPv4",
			in:       list(gw("::ffff:10.0.0.2", 15008)),
			wantKept: list(gw("10.0.0.2", 15008)),
		},
		{
			name:        "one bad address does not cost the peer its good ones",
			in:          list(gw("lb.example.com", 15008), gw("10.0.0.1", 15008)),
			wantKept:    list(gw("10.0.0.1", 15008)),
			wantDropped: []string{"lb.example.com:15008"},
		},
		{
			// The networkName goes on the Gateway as a label value, so an unusable one
			// costs the peer every ambient address, however well-formed. 70 characters,
			// over the 63 a label value allows.
			name:        "an unusable networkName costs the peer all of them",
			networkName: "network-" + strings.Repeat("a", 62),
			in:          list(gw("10.0.0.1", 15008), gw("2001:db8::1", 15008)),
			wantDropped: []string{"10.0.0.1:15008", "[2001:db8::1]:15008"},
		},
		{
			// The same peer with nothing ambient published is left entirely alone: only
			// the ambient path renders networkName as a label value.
			name:        "an unusable networkName alone is not this check's business",
			networkName: "network-" + strings.Repeat("a", 62),
			in:          nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pm := eeCrd.MulticlusterPrivateMetadata{NetworkName: tc.networkName, AmbientGateways: tc.in}

			dropped, reason := sanitizeAmbientGateways(&pm)

			if len(dropped) > 0 && reason == "" {
				t.Errorf("dropped %v without saying why", dropped)
			}
			if !reflect.DeepEqual(dropped, tc.wantDropped) {
				t.Errorf("dropped = %#v, want %#v", dropped, tc.wantDropped)
			}
			if !reflect.DeepEqual(pm.AmbientGateways, tc.wantKept) {
				t.Errorf("kept = %s, want %s", formatGateways(pm.AmbientGateways), formatGateways(tc.wantKept))
			}
		})
	}
}

func formatGateways(gws *[]eeCrd.MulticlusterIngressGateways) string {
	if gws == nil {
		return "<nil>"
	}

	return fmt.Sprintf("%+v", *gws)
}
