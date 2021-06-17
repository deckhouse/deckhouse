package hooks

import (
	"io/ioutil"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: remote_clusters_discovery_public_metadata ::", func() {
	f := HookExecutionConfigInit(`{"istio":{"federation":{},"multicluster":{}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioFederation", false)
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

	Context("Empty cluster, minimal settings, federation and multicluster are enabled", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.federation.enabled", true)
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

	Context("Proper federations and multiclusters only", func() {
		BeforeEach(func() {
			f.ValuesSet(`istio.federation.enabled`, true)
			f.ValuesSet("istio.multicluster.enabled", true)
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: proper-federation-0
spec:
  trustDomain: "p.f0"
  metadataEndpoint: "file:///tmp/proper-federation-0/"
status: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: proper-multicluster-0
spec:
  metadataEndpoint: "file:///tmp/proper-multicluster-0/"
status: {}
`))
			_ = os.MkdirAll("/tmp/proper-federation-0/public", 0755)
			ioutil.WriteFile("/tmp/proper-federation-0/public/public.json", []byte(`{"a":"b"}`), 0644)
			_ = os.MkdirAll("/tmp/proper-multicluster-0/public", 0755)
			ioutil.WriteFile("/tmp/proper-multicluster-0/public/public.json", []byte(`{"x":"y"}`), 0644)

			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())

			stderrBuff := string(f.Session.Err.Contents())
			Expect(stderrBuff).To(Equal(""))

			tf0, err := time.Parse(time.RFC3339, f.KubernetesGlobalResource("IstioFederation", "proper-federation-0").Field("status.metadataCache.publicLastFetchTimestamp").String())
			Expect(err).ShouldNot(HaveOccurred())
			tm0, err := time.Parse(time.RFC3339, f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-0").Field("status.metadataCache.publicLastFetchTimestamp").String())
			Expect(err).ShouldNot(HaveOccurred())

			Expect(tf0).Should(BeTemporally("~", time.Now().UTC(), time.Minute))
			Expect(tm0).Should(BeTemporally("~", time.Now().UTC(), time.Minute))

			Expect(f.KubernetesGlobalResource("IstioFederation", "proper-federation-0").Field("status.metadataCache.public").String()).To(MatchJSON(`{"a":"b"}`))
			Expect(f.KubernetesGlobalResource("IstioMulticluster", "proper-multicluster-0").Field("status.metadataCache.public").String()).To(MatchJSON(`{"x":"y"}`))
		})
	})

	Context("Improper federation and multicluster", func() {
		BeforeEach(func() {
			f.ValuesSet(`istio.federation.enabled`, true)
			f.ValuesSet(`istio.multicluster.enabled`, true)
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: improper-federation-0
spec:
  trustDomain: "i.f0"
  metadataEndpoint: "https://some-improper-hostname-f/metadata/"
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: improper-multicluster-0
spec:
  metadataEndpoint: "https://some-improper-hostname-m/metadata/"
`))

			f.RunHook()
		})

		It("Hook must execute successfully with proper warnings", func() {
			Expect(f).To(ExecuteSuccessfully())

			stderrBuff := string(f.Session.Err.Contents())
			Expect(stderrBuff).Should(ContainSubstring(`ERROR: Cannot fetch public metadata endpoint https://some-improper-hostname-f/metadata/public/public.json for IstioFederation improper-federation-0.`))
			Expect(stderrBuff).Should(ContainSubstring(`ERROR: Cannot fetch public metadata endpoint https://some-improper-hostname-m/metadata/public/public.json for IstioMulticluster improper-multicluster-0.`))
		})
	})
})
