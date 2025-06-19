/*
Copyright 2025 Flant JSC

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
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	nsYaml = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-cloud-instance-manager
`

	lsYaml = `
---
apiVersion: coordination.k8s.io/v1
kind: Lease
metadata:
  name: faf94607.cluster.x-k8s.io
  namespace: d8-cloud-instance-manager
`
)

var _ = Describe("node-manager :: hooks :: remove_old_caps_lease_migration ::", func() {
	f := HookExecutionConfigInit("", "")

	Context("An empty cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunGoHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with old caps lease installed", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext())

			createNs(f.KubeClient(), nsYaml)
			createLease(f.KubeClient(), lsYaml)

			f.RunGoHook()
		})

		It("Lease must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			_, err := f.KubeClient().CoreV1().Namespaces().Get(context.TODO(), d8CapsNs, metav1.GetOptions{})
			Expect(err).To(BeNil())
			_, err = f.KubeClient().CoordinationV1().Leases(d8CapsNs).Get(context.TODO(), d8CapsLeaseNameOld, metav1.GetOptions{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})
})

func createNs(kubeClient client.KubeClient, spec string) {
	ns := new(corev1.Namespace)
	if err := yaml.Unmarshal([]byte(spec), ns); err != nil {
		panic(err)
	}
	_, _ = kubeClient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
}

func createLease(kubeClient client.KubeClient, spec string) {
	ls := new(coordinationv1.Lease)
	if err := yaml.Unmarshal([]byte(spec), ls); err != nil {
		panic(err)
	}
	_, _ = kubeClient.CoordinationV1().Leases(d8CapsNs).Create(context.TODO(), ls, metav1.CreateOptions{})
}
