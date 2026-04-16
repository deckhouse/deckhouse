/*
Copyright 2026 Flant JSC

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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/modules/402-ingress-nginx/hooks/internal"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("ingress-nginx :: hooks :: migrate_ingress_nginx_controller_api_version ::", func() {
	f := HookExecutionConfigInit("", "")
	f.RegisterCRD(internal.IngressNginxControllerGVR.Group, internal.IngressNginxControllerGVR.Version, "IngressNginxController", false)

	Context("An empty cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunGoHook()
		})

		It("executes successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with ingress controllers requiring rewrite", func() {
		BeforeEach(func() {
			f.KubeStateSet(controllersYAML)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunGoHook()
		})

		It("marks controllers as migrated through v1 API", func() {
			Expect(f).To(ExecuteSuccessfully())

			mainController, err := f.KubeClient().Dynamic().Resource(internal.IngressNginxControllerGVR).Get(context.TODO(), "main", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(mainController.GetAnnotations()).To(HaveKeyWithValue(ingressNginxControllerAPIVersionMigrationAnnotation, ingressNginxControllerAPIVersionTarget))

			canaryController, err := f.KubeClient().Dynamic().Resource(internal.IngressNginxControllerGVR).Get(context.TODO(), "canary", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(canaryController.GetAnnotations()).To(HaveKeyWithValue(ingressNginxControllerAPIVersionMigrationAnnotation, ingressNginxControllerAPIVersionTarget))
		})
	})
})

const controllersYAML = `
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec: {}
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: canary
  annotations:
    ingress-nginx.deckhouse.io/migrated-api-version: deckhouse.io/v1alpha1
spec: {}
`
