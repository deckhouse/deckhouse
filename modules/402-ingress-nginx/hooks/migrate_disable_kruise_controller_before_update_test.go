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
	anotherDeployment          = "test-nginx"
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
  replicas: %d
  template:
    metadata:
      labels:
        app: kruise
        control-plane: controller-manager
    spec:
      containers:
      - command:
        - /manager
        image: mockImage:latest
        name: kruise
`
	anotherDeploymentDefinition = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
spec:
  selector:
    matchLabels:
      app: test-nginx
  replicas: %d
  template:
    metadata:
      labels:
        app: test-nginx
    spec:
      containers:
      - name: test-nginx
        image: nginx:latest
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
	var kruiseDeploymentMock = fmt.Sprintf(kruiseControllerDefinition, targetDeployment, "some-annotation", replicasBefore)
	var kruiseDeploymentAnnotatedMock = fmt.Sprintf(kruiseControllerDefinition, targetDeployment, kruisePatchAnnotation, replicasBefore)
	var anotherDeploymentMock = fmt.Sprintf(anotherDeploymentDefinition, anotherDeployment, replicasBefore)

	getDeployment := func(namespace, name string) (*appsv1.Deployment, error) {
		deployment, err := dependency.TestDC.MustGetK8sClient().AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		return deployment, err
	}

	getDeploymentReplicas := func(namespace, name string) (int32, error) {
		deployment, err := getDeployment(namespace, name)
		if err != nil {
			return 0, err
		}
		return *deployment.Spec.Replicas, nil
	}

	getDeploymentAnnotations := func(namespace, name string) (map[string]string, error) {
		deployment, err := getDeployment(namespace, name)
		if err != nil {
			return nil, err
		}
		return deployment.ObjectMeta.GetAnnotations(), nil
	}

	assertScale := func(namespace, name string, replicas int32) {
		r, err := getDeploymentReplicas(namespace, name)
		Expect(err).ToNot(HaveOccurred())
		Expect(r).To(Equal(replicas))
	}

	checkAnnotationInPlace := func(namespace, name string) {
		annotations, err := getDeploymentAnnotations(namespace, name)
		Expect(err).ToNot(HaveOccurred())
		Expect(annotations).To(HaveKey(kruisePatchAnnotation))
	}

	checkAnnotationNotInPlace := func(namespace, name string) {
		annotations, err := getDeploymentAnnotations(namespace, name)
		Expect(err).ToNot(HaveOccurred())
		Expect(annotations).ToNot(HaveKey(kruisePatchAnnotation))
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
			createMockDeployment(anotherDeploymentMock)
		})

		It(fmt.Sprintf("Before running the hook, both deployments should have replicas set to %d", replicasBefore), func() {
			Expect(f).To(ExecuteSuccessfully())

			assertScale(targetNamespace, targetDeployment, replicasBefore)

			assertScale(targetNamespace, anotherDeployment, replicasBefore)
		})
	})

	Context("", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			createMockDeployment(kruiseDeploymentMock)
			createMockDeployment(anotherDeploymentMock)
			f.RunHook()
		})

		It(fmt.Sprintf("After running the hook, %s deployment should have replicas set to %d and annotated with %s, and %s deployment should be unchanged", targetDeployment, replicasAfter, kruisePatchAnnotation, anotherDeployment), func() {
			Expect(f).To(ExecuteSuccessfully())

			assertScale(targetNamespace, targetDeployment, replicasAfter)

			checkAnnotationInPlace(targetNamespace, targetDeployment)

			assertScale(targetNamespace, anotherDeployment, replicasBefore)

			checkAnnotationNotInPlace(targetNamespace, anotherDeployment)
		})
	})

	Context("", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			createMockDeployment(kruiseDeploymentMock)
			createMockDeployment(anotherDeploymentMock)
			f.RunHook()
		})

		It(fmt.Sprintf("After running the hook, %s deployment should have replicas set to %d and %s annotation, and %s deployment should be unchanged", targetDeployment, replicasAfter, kruisePatchAnnotation, anotherDeployment), func() {
			Expect(f).To(ExecuteSuccessfully())

			assertScale(targetNamespace, targetDeployment, replicasAfter)

			checkAnnotationInPlace(targetNamespace, targetDeployment)

			assertScale(targetNamespace, anotherDeployment, replicasBefore)

			checkAnnotationNotInPlace(targetNamespace, anotherDeployment)
		})
	})

	Context("", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			createMockDeployment(kruiseDeploymentAnnotatedMock)
			createMockDeployment(anotherDeploymentMock)
			f.RunHook()
		})

		It(fmt.Sprintf("After running the hook, annotated %s deployment should be unchanged, and %s deployment should be unchanged", targetDeployment, anotherDeployment), func() {
			Expect(f).To(ExecuteSuccessfully())

			assertScale(targetNamespace, targetDeployment, replicasBefore)

			checkAnnotationInPlace(targetNamespace, targetDeployment)

			assertScale(targetNamespace, anotherDeployment, replicasBefore)

			checkAnnotationNotInPlace(targetNamespace, anotherDeployment)

		})
	})
})
