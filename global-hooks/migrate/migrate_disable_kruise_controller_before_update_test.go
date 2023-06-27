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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	anotherDeployment = "test-nginx"
	replicasBefore = 3
	replicasAfter = 0
	kruiseControllerDefinition = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
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

func createMockDeployment(namespace, cfg string) {
	var dep *appsv1.Deployment

	err := yaml.Unmarshal([]byte(cfg), &dep)
	if err != nil {
		panic(err)
	}
	_, err = dependency.TestDC.MustGetK8sClient().AppsV1().Deployments(namespace).Create(context.TODO(), dep, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

var _ = Describe("Global :: migrate_disable_kruise_controller_before_update ::", func() {
	var kruiseDeploymentMock = fmt.Sprintf(kruiseControllerDefinition, targetDeployment, replicasBefore)
	var anotherDeploymentMock = fmt.Sprintf(anotherDeploymentDefinition, anotherDeployment, replicasBefore)

	getDeploymentReplicas := func(namespace, name string) (int32, error) {
		deployment, err := dependency.TestDC.MustGetK8sClient().AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return 0, err
		}
		return *deployment.Spec.Replicas, nil
        }

	assertScale := func(namespace, name string, replicas int32) {
		r, err := getDeploymentReplicas(namespace, name)
		Expect(err).ToNot(HaveOccurred())
		Expect(r).To(Equal(replicas))
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
			createMockDeployment(targetNamespace, kruiseDeploymentMock)
			createMockDeployment(targetNamespace, anotherDeploymentMock)
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
			createMockDeployment(targetNamespace, kruiseDeploymentMock)
			createMockDeployment(targetNamespace, anotherDeploymentMock)
			f.RunHook()
		})

		It(fmt.Sprintf("After running the hook, %s deployment should have replicas set to %d, and %s deployment should be unchanged", targetDeployment, replicasAfter, anotherDeployment), func() {
			Expect(f).To(ExecuteSuccessfully())

			assertScale(targetNamespace, targetDeployment, replicasAfter)

			assertScale(targetNamespace, anotherDeployment, replicasBefore)
		})
	})
})
