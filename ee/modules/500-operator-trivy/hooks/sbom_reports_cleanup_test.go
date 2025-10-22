/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var (
	nsDefault = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: default
`
	nsOperatorTrivy = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-operator-trivy
  annotations:
    meta.helm.sh/release-name: operator-trivy
    meta.helm.sh/release-namespace: d8-system
  labels:
    app.kubernetes.io/managed-by: Helm
    extended-monitoring.deckhouse.io/enabled: ""
    heritage: deckhouse
    kubernetes.io/metadata.name: d8-operator-trivy
    module: operator-trivy
    prometheus.deckhouse.io/rules-watcher-enabled: "true"
`

	nsOperatorTrivyCleanedUp = nsOperatorTrivy + `
    sbom-cleaned-up: "true"
`

	sbom1Yaml = `
---
apiVersion: aquasecurity.github.io/v1alpha1
kind: SbomReport
metadata:
  name: job-echo
  namespace: default
report:
  artifact:
    digest: sha256:6013ae1a63c2ee58a8949f03c6366a3ef6a2f386a7db27d86de2de965e9f450b
    repository: library/ubuntu
    tag: "20.04"
`

	sbom2Yaml = `
---
apiVersion: aquasecurity.github.io/v1alpha1
kind: SbomReport
metadata:
  name: job-alpha
  namespace: default
report:
  artifact:
    digest: sha256:6013ae1a63c2ee58a8949f03c6366a3ef6a2f386a7db27d86de2de965e9f450b
    repository: library/ubuntu
    tag: "20.04"
`

	moduleConfigSBOMDisabled = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: operator-trivy
spec:
  enabled: true
  settings:
    disableSBOMGeneration: true
`

	moduleConfigSBOMEnabled = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: operator-trivy
spec:
  enabled: true
  settings:
    disableSBOMGeneration: false
`
)

var _ = Describe("Modules :: operator-trivy :: hooks :: sbom reports cleanup ::", func() {

	f := HookExecutionConfigInit("", "")
	f.RegisterCRD("aquasecurity.github.io", "v1alpha1", "SbomReport", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context(":: empty cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context(":: empty cluster with ns", func() {
		BeforeEach(func() {
			f.KubeStateSet(nsDefault + nsOperatorTrivy)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Namespace unchanged", func() {
			Expect(f).To(ExecuteSuccessfully())
			ns := f.KubernetesResource("Namespace", "", "d8-operator-trivy")
			Expect(ns.Exists()).To(BeTrue())
			Expect(ns.ToYaml()).To(MatchYAML(nsOperatorTrivy))
		})

	})
	Context(":: ns plus sboms", func() {
		BeforeEach(func() {
			f.KubeStateSet(nsDefault + nsOperatorTrivy + sbom1Yaml + sbom2Yaml)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Namespace unchanged, sboms in place", func() {
			Expect(f).To(ExecuteSuccessfully())
			ns := f.KubernetesResource("Namespace", "", "d8-operator-trivy")
			Expect(ns.Exists()).To(BeTrue())
			Expect(ns.ToYaml()).To(MatchYAML(nsOperatorTrivy))
			_, err := f.KubeClient().Dynamic().Resource(sbomGVR).Namespace("default").Get(context.TODO(), "job-echo", metav1.GetOptions{})
			Expect(err).To(BeNil())
			_, err = f.KubeClient().Dynamic().Resource(sbomGVR).Namespace("default").Get(context.TODO(), "job-alpha", metav1.GetOptions{})
			Expect(err).To(BeNil())
		})
	})

	Context(":: ns plus sboms and module config set to disable sboms", func() {
		BeforeEach(func() {
			f.KubeStateSet(nsDefault + nsOperatorTrivy + sbom1Yaml + sbom2Yaml + moduleConfigSBOMDisabled)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Namespace labeled, sboms deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			ns := f.KubernetesResource("Namespace", "", "d8-operator-trivy")
			Expect(ns.Exists()).To(BeTrue())
			Expect(ns.ToYaml()).To(MatchYAML(nsOperatorTrivyCleanedUp))
			_, err := f.KubeClient().Dynamic().Resource(sbomGVR).Namespace("default").Get(context.TODO(), "job-echo", metav1.GetOptions{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
			_, err = f.KubeClient().Dynamic().Resource(sbomGVR).Namespace("default").Get(context.TODO(), "job-alpha", metav1.GetOptions{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})
	})

	Context(":: labeled ns plus sboms and module config set to disable sboms", func() {
		BeforeEach(func() {
			f.KubeStateSet(nsDefault + nsOperatorTrivyCleanedUp + sbom1Yaml + sbom2Yaml + moduleConfigSBOMDisabled)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Namespace labeled, sboms unchanged", func() {
			Expect(f).To(ExecuteSuccessfully())
			ns := f.KubernetesResource("Namespace", "", "d8-operator-trivy")
			Expect(ns.Exists()).To(BeTrue())
			Expect(ns.ToYaml()).To(MatchYAML(nsOperatorTrivyCleanedUp))
			_, err := f.KubeClient().Dynamic().Resource(sbomGVR).Namespace("default").Get(context.TODO(), "job-echo", metav1.GetOptions{})
			Expect(err).To(BeNil())
			_, err = f.KubeClient().Dynamic().Resource(sbomGVR).Namespace("default").Get(context.TODO(), "job-alpha", metav1.GetOptions{})
			Expect(err).To(BeNil())
		})
	})

	Context(":: labeled ns plus sboms and module config set to enable sboms", func() {
		BeforeEach(func() {
			f.KubeStateSet(nsDefault + nsOperatorTrivyCleanedUp + sbom1Yaml + sbom2Yaml + moduleConfigSBOMEnabled)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Namespace unlabeled, sboms unchanged", func() {
			Expect(f).To(ExecuteSuccessfully())
			ns := f.KubernetesResource("Namespace", "", "d8-operator-trivy")
			Expect(ns.Exists()).To(BeTrue())
			Expect(ns.ToYaml()).To(MatchYAML(nsOperatorTrivy))
			_, err := f.KubeClient().Dynamic().Resource(sbomGVR).Namespace("default").Get(context.TODO(), "job-echo", metav1.GetOptions{})
			Expect(err).To(BeNil())
			_, err = f.KubeClient().Dynamic().Resource(sbomGVR).Namespace("default").Get(context.TODO(), "job-alpha", metav1.GetOptions{})
			Expect(err).To(BeNil())
		})
	})

})
