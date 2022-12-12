/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/square/go-jose/v3"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
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
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0].Action).Should(Equal("expire"))
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
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0].Action).Should(Equal("expire"))
		})
	})

	Context("Proper multiclusters only", func() {
		var bearerTokens = map[string]string{}

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
      apiHost: some-outdatad-api.host
      networkName: some-outdated-networkname
    public:
      clusterUUID: bad-cluster-uuid # should be changed
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
`))

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
                          "apiHost": "api-host-0",
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
			}
			dependency.TestDC.HTTPClient.DoMock.
				Set(func(req *http.Request) (rp1 *http.Response, err error) {
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
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			tPub0, err := time.Parse(time.RFC3339, f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-0").Field("status.metadataCache.publicLastFetchTimestamp").String())
			Expect(err).ShouldNot(HaveOccurred())
			tPub1, err := time.Parse(time.RFC3339, f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-1").Field("status.metadataCache.publicLastFetchTimestamp").String())
			Expect(err).ShouldNot(HaveOccurred())
			tPub2, err := time.Parse(time.RFC3339, f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-2").Field("status.metadataCache.publicLastFetchTimestamp").String())
			Expect(err).ShouldNot(HaveOccurred())

			tPriv0, err := time.Parse(time.RFC3339, f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-0").Field("status.metadataCache.privateLastFetchTimestamp").String())
			Expect(err).ShouldNot(HaveOccurred())
			tPriv1, err := time.Parse(time.RFC3339, f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-1").Field("status.metadataCache.privateLastFetchTimestamp").String())
			Expect(err).ShouldNot(HaveOccurred())
			tPriv2, err := time.Parse(time.RFC3339, f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-2").Field("status.metadataCache.privateLastFetchTimestamp").String())
			Expect(err).ShouldNot(HaveOccurred())

			Expect(tPub0).Should(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tPub1).Should(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tPub2).Should(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tPriv0).Should(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tPriv1).Should(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tPriv2).Should(BeTemporally("~", time.Now().UTC(), time.Minute))

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

			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-0").Field("status.metadataCache.private.apiHost").String()).To(Equal("api-host-0"))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-1").Field("status.metadataCache.private.apiHost").String()).To(Equal("api-host-1"))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-2").Field("status.metadataCache.private.apiHost").String()).To(Equal("api-host-2"))

			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-0").Field("status.metadataCache.private.networkName").String()).To(Equal("network-name-0"))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-1").Field("status.metadataCache.private.networkName").String()).To(Equal("network-name-1"))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-2").Field("status.metadataCache.private.networkName").String()).To(Equal("network-name-2"))

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
				Action: "expire",
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(0.0),
				Labels: map[string]string{
					"multicluster_name": "proper-multicluster-0",
					"endpoint":          "https://proper-hostname-0/metadata/public/public.json",
				},
			}))
			Expect(m[2]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(0.0),
				Labels: map[string]string{
					"multicluster_name": "proper-multicluster-0",
					"endpoint":          "https://proper-hostname-0/metadata/private/multicluster.json",
				},
			}))
			Expect(m[3]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(0.0),
				Labels: map[string]string{
					"multicluster_name": "proper-multicluster-1",
					"endpoint":          "https://proper-hostname-1/metadata/public/public.json",
				},
			}))
			Expect(m[4]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(0.0),
				Labels: map[string]string{
					"multicluster_name": "proper-multicluster-1",
					"endpoint":          "https://proper-hostname-1/metadata/private/multicluster.json",
				},
			}))
			Expect(m[5]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(0.0),
				Labels: map[string]string{
					"multicluster_name": "proper-multicluster-2",
					"endpoint":          "https://proper-hostname-2/metadata/public/public.json",
				},
			}))
			Expect(m[6]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(0.0),
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
			f.BindingContexts.Set(f.KubeStateSet(`
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
`))

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
				Set(func(req *http.Request) (rp1 *http.Response, err error) {
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

			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("cannot fetch public metadata endpoint https://public-internal-error/metadata/public/public.json for IstioMulticluster public-internal-error (HTTP Code 500)"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("cannot unmarshal public metadata endpoint https://public-bad-json/metadata/public/public.json for IstioMulticluster public-bad-json, error: unexpected end of JSON input"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("bad public metadata format in endpoint https://public-wrong-format/metadata/public/public.json for IstioMulticluster public-wrong-format"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("cannot fetch private metadata endpoint https://private-internal-error/metadata/private/multicluster.json for IstioMulticluster private-internal-error (HTTP Code 500)"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("cannot unmarshal private metadata endpoint https://private-bad-json/metadata/private/multicluster.json for IstioMulticluster private-bad-json, error: unexpected end of JSON input"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("bad private metadata format in endpoint https://private-wrong-format/metadata/private/multicluster.json for IstioMulticluster private-wrong-format"))

			Expect(f.KubernetesGlobalResource("IstioMulticluster", "public-internal-error").Field("status").String()).To(MatchJSON("{}"))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "public-bad-json").Field("status").String()).To(MatchJSON("{}"))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "public-wrong-format").Field("status").String()).To(MatchJSON("{}"))

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
				Action: "expire",
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(0.0),
				Labels: map[string]string{
					"multicluster_name": "private-bad-json",
					"endpoint":          "https://private-bad-json/metadata/public/public.json",
				},
			}))
			Expect(m[2]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"multicluster_name": "private-bad-json",
					"endpoint":          "https://private-bad-json/metadata/private/multicluster.json",
				},
			}))
			Expect(m[3]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(0.0),
				Labels: map[string]string{
					"multicluster_name": "private-internal-error",
					"endpoint":          "https://private-internal-error/metadata/public/public.json",
				},
			}))
			Expect(m[4]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"multicluster_name": "private-internal-error",
					"endpoint":          "https://private-internal-error/metadata/private/multicluster.json",
				},
			}))
			Expect(m[5]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(0.0),
				Labels: map[string]string{
					"multicluster_name": "private-wrong-format",
					"endpoint":          "https://private-wrong-format/metadata/public/public.json",
				},
			}))
			Expect(m[6]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"multicluster_name": "private-wrong-format",
					"endpoint":          "https://private-wrong-format/metadata/private/multicluster.json",
				},
			}))
			Expect(m[7]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"multicluster_name": "public-bad-json",
					"endpoint":          "https://public-bad-json/metadata/public/public.json",
				},
			}))
			Expect(m[8]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"multicluster_name": "public-internal-error",
					"endpoint":          "https://public-internal-error/metadata/public/public.json",
				},
			}))
			Expect(m[9]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMetricName,
				Group:  multiclusterMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"multicluster_name": "public-wrong-format",
					"endpoint":          "https://public-wrong-format/metadata/public/public.json",
				},
			}))
		})
	})
})
