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
	base                       = "registry/kruise"
	oldHash                    = "sha256:80cf1b1973f6739e76a81b2b521bdd43c832be29b682feb9a94b2c10f268adab"
	newHash                    = "sha256:9558d2ec1e8ac5f41c1a03f174d5e20920ce9637d957016043bcfa3d45169792"
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
    spec:
      containers:
      - command:
        - /manager
        image: %s
        name: kruise
      - command:
        - /metrics
        image: metrics
        name: metrics
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
	var kruiseDeploymentMock = fmt.Sprintf(kruiseControllerDefinition, targetDeployment, "some-annotation", fmt.Sprintf("%s@%s", base, oldHash))
	var kruiseDeploymentAnnotatedMock = fmt.Sprintf(kruiseControllerDefinition, targetDeployment, kruisePatchAnnotation, fmt.Sprintf("%s@%s", base, oldHash))

	getDeployment := func() (*appsv1.Deployment, error) {
		deployment, err := dependency.TestDC.MustGetK8sClient().AppsV1().Deployments(targetNamespace).Get(context.TODO(), targetDeployment, metav1.GetOptions{})
		return deployment, err
	}

	getDeploymentAnnotations := func() (map[string]string, error) {
		deployment, err := getDeployment()
		if err != nil {
			return nil, err
		}
		return deployment.ObjectMeta.GetAnnotations(), nil
	}

	updateDeploymentStatus := func() {
		deployment, _ := getDeployment()
		deployment.Status.ReadyReplicas = *deployment.Spec.Replicas
		_, _ = dependency.TestDC.MustGetK8sClient().AppsV1().Deployments(targetNamespace).UpdateStatus(context.TODO(), deployment, metav1.UpdateOptions{})
	}

	getKruiseImage := func() (string, error) {
		var image string
		deployment, err := getDeployment()
		if err != nil {
			return "", err
		}
		for _, container := range deployment.Spec.Template.Spec.Containers {
			if container.Name == "kruise" {
				image = container.Image
				break
			}
		}

		return image, nil
	}

	assertAnnotationInPlace := func() {
		annotations, err := getDeploymentAnnotations()
		Expect(err).ToNot(HaveOccurred())
		Expect(annotations).To(HaveKey(kruisePatchAnnotation))
	}

	assertAnnotationNotInPlace := func() {
		annotations, err := getDeploymentAnnotations()
		Expect(err).ToNot(HaveOccurred())
		Expect(annotations).ToNot(HaveKey(kruisePatchAnnotation))
	}

	assertImage := func(image string) {
		kruiseImage, err := getKruiseImage()
		Expect(err).ToNot(HaveOccurred())
		Expect(kruiseImage).To(Equal(image))
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

			assertAnnotationNotInPlace()
		})
	})

	Context("", func() {
		f := HookExecutionConfigInit(fmt.Sprintf(`{"global": {"modulesImages":{"registry":{"base": "%s"},"digests":{"ingressNginx":{"kruise": "%s"}}}}}`, base, newHash), ``)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			createMockDeployment(kruiseDeploymentMock)
			updateDeploymentStatus()
			f.RunHook()
		})

		It(fmt.Sprintf("After running the hook, %s deployment should have %s annotations and image set to %s", targetDeployment, kruisePatchAnnotation, fmt.Sprintf("%s@%s", base, newHash)), func() {
			Expect(f).To(ExecuteSuccessfully())

			assertAnnotationInPlace()

			assertImage(fmt.Sprintf("%s@%s", base, newHash))
		})
	})

	Context("", func() {
		f := HookExecutionConfigInit(fmt.Sprintf(`{"global": {"modulesImages":{"registry":{"base": "%s"},"digests":{"ingressNginx":{"kruise": "%s"}}}}}`, base, newHash), ``)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			createMockDeployment(kruiseDeploymentAnnotatedMock)
			f.RunHook()
		})

		It(fmt.Sprintf("After running the hook, annotated %s deployment should have %s annotations and image unchanged", targetDeployment, kruisePatchAnnotation), func() {
			Expect(f).To(ExecuteSuccessfully())

			assertAnnotationInPlace()

			assertImage(fmt.Sprintf("%s@%s", base, oldHash))

		})
	})

	Context("", func() {
		f := HookExecutionConfigInit(fmt.Sprintf(`{"global": {"modulesImages":{"registry":{"base": "%s"},"digests":{"ingressNginx":{"kruise": "%s"}}}}}`, base, oldHash), ``)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			createMockDeployment(kruiseDeploymentMock)
			f.RunHook()
		})

		It(fmt.Sprintf("After running the hook, not annotated %s deployment with correct image should have image unchanged", targetDeployment), func() {
			Expect(f).To(ExecuteSuccessfully())

			assertAnnotationNotInPlace()

			assertImage(fmt.Sprintf("%s@%s", base, oldHash))

		})
	})
})
