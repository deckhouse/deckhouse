/*
Copyright 2023 Flant JSC

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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	someDate                   = "2022-11-29T16:33:08+03:30"
	replicasBefore             = 3
	replicasAfter              = 0
	kruiseControllerDefinition = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  annotations:
    %s: ""
spec:
  selector:
    matchLabels:
      app: kruise
      control-plane: controller-manager
  replicas: 1
  template:
    metadata:
      labels:
        app: kruise
        control-plane: controller-manager
      annotations:
        %s: %s
    spec:
      containers:
      - command:
        - /manager
        image: mockImage:latest
        name: kruise
`
)

func createMockDeployment(cfg string) {
	var dep *appsv1.Deployment

	err := yaml.Unmarshal([]byte(cfg), &dep)
	if err != nil {
		panic(err)
	}
	_, err = dependency.TestDC.MustGetK8sClient().AppsV1().Deployments(targetNamespace).Create(context.TODO(), dep, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

var _ = Describe("Global :: migrate_disable_kruise_controller_before_update ::", func() {
	var kruiseDeploymentMock = fmt.Sprintf(kruiseControllerDefinition, targetDeployment, "some-annotation", "some-annotation", someDate)
	var kruiseDeploymentAnnotatedMock = fmt.Sprintf(kruiseControllerDefinition, targetDeployment, kruisePatchAnnotation, restartAnnotation, someDate)

	getDeployment := func(namespace, name string) (*appsv1.Deployment, error) {
		deployment, err := dependency.TestDC.MustGetK8sClient().AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		return deployment, err
	}

	getDeploymentAnnotations := func(namespace, name string) (map[string]string, error) {
		deployment, err := getDeployment(namespace, name)
		if err != nil {
			return nil, err
		}
		return deployment.ObjectMeta.GetAnnotations(), nil
	}

	getDeploymentTemplateAnnotations := func(namespace, name string) (map[string]string, error) {
		deployment, err := getDeployment(namespace, name)
		if err != nil {
			return nil, err
		}
		return deployment.Spec.Template.ObjectMeta.GetAnnotations(), nil
	}

	assertDeploymentAnnotationInPlace := func(namespace, name string) {
		annotations, err := getDeploymentAnnotations(namespace, name)
		Expect(err).ToNot(HaveOccurred())
		Expect(annotations).To(HaveKey(kruisePatchAnnotation))
	}

	assertDeploymentAnnotationNotInPlace := func(namespace, name string) {
		annotations, err := getDeploymentAnnotations(namespace, name)
		Expect(err).ToNot(HaveOccurred())
		Expect(annotations).ToNot(HaveKey(kruisePatchAnnotation))
	}

	assertTemplateAnnotationNotUpdated := func(namespace, name, annotation string) {
		templateAnnotations, err := getDeploymentTemplateAnnotations(namespace, name)
		Expect(err).ToNot(HaveOccurred())
		Expect(templateAnnotations).To(HaveKey(restartAnnotation))
		Expect(templateAnnotations[restartAnnotation]).To(BeEquivalentTo(annotation))
	}

	assertTemplateAnnotationUpdated := func(namespace, name, annotation string) {
		templateAnnotations, err := getDeploymentTemplateAnnotations(namespace, name)
		Expect(err).ToNot(HaveOccurred())
		Expect(templateAnnotations).To(HaveKey(restartAnnotation))
		Expect(templateAnnotations[restartAnnotation]).ToNot(BeEquivalentTo(annotation))
	}

	Context("Empty cluster", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook should not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			createMockDeployment(kruiseDeploymentMock)
		})

		It(fmt.Sprintf("Before running the hook, %s deployments should't have %s annotation", targetDeployment, kruisePatchAnnotation), func() {
			Expect(f).To(ExecuteSuccessfully())

			assertDeploymentAnnotationNotInPlace(targetNamespace, targetDeployment)
		})
	})

	Context("", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			createMockDeployment(kruiseDeploymentMock)
			f.RunHook()
		})

		It(fmt.Sprintf("After running the hook, %s deployment should have %s annotations and %s .spec.template annotation", targetDeployment, kruisePatchAnnotation, restartAnnotation), func() {
			Expect(f).To(ExecuteSuccessfully())

			assertDeploymentAnnotationInPlace(targetNamespace, targetDeployment)

			assertTemplateAnnotationUpdated(targetNamespace, targetDeployment, someDate)
		})
	})

	Context("", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			createMockDeployment(kruiseDeploymentAnnotatedMock)
			f.RunHook()
		})

		It(fmt.Sprintf("After running the hook, annotated %s deployment shouldn't be restarted", targetDeployment), func() {
			Expect(f).To(ExecuteSuccessfully())

			assertDeploymentAnnotationInPlace(targetNamespace, targetDeployment)

			assertTemplateAnnotationNotUpdated(targetNamespace, targetDeployment, someDate)

		})
	})
})
