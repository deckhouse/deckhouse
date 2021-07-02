/*
Copyright 2021 Flant CJSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"context"
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("ingress-nginx :: hooks :: migrate_load_balancer_before ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"defaultControllerVersion": 0.25, "internal": {"webhookCertificates":{}}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", false)

	dsControllerMainYAML := `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: controller-main
  namespace: d8-ingress-nginx
  labels:
    name: main
    app: controller
  annotations:
    ingress-nginx-controller.deckhouse.io/inlet: LoadBalancer
status:
  desiredNumberScheduled: 2
`
	ingressControllerMainYAML := `
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
`
	dsControllerOtherYAML := `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: controller-other
  namespace: d8-ingress-nginx
  labels:
    name: main
    app: controller
  annotations:
    ingress-nginx-controller.deckhouse.io/inlet: HostPort
`
	testdata, _ := ioutil.ReadFile("testdata/controller-release-secret-with-ds.yaml")
	var secretRelease *corev1.Secret
	_ = yaml.Unmarshal(testdata, &secretRelease)

	Context("Cluster with ingress controller and its DaemonSet", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(dsControllerMainYAML + ingressControllerMainYAML))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			_, _ = f.KubeClient().CoreV1().Secrets("d8-system").Create(context.TODO(), secretRelease, metav1.CreateOptions{})

			f.RunHook()
		})

		It("must be execute successfully, set replicas and chang helm release secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			ingressControllerMain := f.KubernetesResource("IngressNginxController", "", "main")
			Expect(ingressControllerMain.Exists()).To(BeTrue())
			Expect(ingressControllerMain.Field("spec.minReplicas").Int()).To(Equal(int64(4)))
			Expect(ingressControllerMain.Field("spec.maxReplicas").Int()).To(Equal(int64(12)))

			releaseSecret, _ := f.KubeClient().CoreV1().Secrets("d8-system").Get(context.TODO(), "sh.helm.release.v1.ingress-nginx.v66", metav1.GetOptions{})
			Expect(releaseSecret).ToNot(Equal(secretRelease))

			initialRelease, _ := ParseReleaseSecretToJSON(secretRelease)
			finalRelease, _ := ParseReleaseSecretToJSON(releaseSecret)

			initialManifest := initialRelease["manifest"]
			finalManifest := finalRelease["manifest"]

			delete(initialRelease, "manifest")
			delete(finalRelease, "manifest")

			Expect(initialRelease).To(Equal(finalRelease))
			Expect(initialManifest).ToNot(Equal(finalManifest))
		})
	})

	Context("Cluster with ingress controller DaemonSet, not suitable for migration", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(dsControllerOtherYAML))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())

			f.RunHook()
		})

		It("must be execute successfully, set replicas and chang helm release secret", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

})
