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
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakecorev1 "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: documentation :: hooks :: s3_connection", func() {
	f := HookExecutionConfigInit(
		`{"documentation": {"internal": {}, "moduleStorage": {"s3": {"type": "External", "external": {"secretRef": {"namespace": "storage", "name": "documentation-s3"}}}}}}`,
		`{"documentation":{}}`,
	)
	// KubeStateSet makes a new fake cluster: Secrets are created after it.
	BeforeEach(func() { f.KubeStateSet("") })
	run := func() {
		f.BindingContexts.Set(f.GenerateBeforeHelmContext())
		f.RunHook()
		Expect(f).To(ExecuteSuccessfully())
	}

	It("Copies the Secret of the user from its namespace", func() {
		secrets := dependency.TestDC.MustGetK8sClient().CoreV1().Secrets("storage")
		_, err := secrets.Create(context.TODO(), &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "storage", Name: "documentation-s3"},
			Data:       map[string][]byte{"S3_BUCKET": []byte("documentation"), "AWS_ACCESS_KEY_ID": []byte("AKID")},
		}, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		run()
		Expect(f.ValuesGet("documentation.internal.s3Connection").String()).To(MatchJSON(`{"S3_BUCKET":"ZG9jdW1lbnRhdGlvbg==","AWS_ACCESS_KEY_ID":"QUtJRA=="}`))

		// A failure of the API keeps the last copy.
		core := dependency.TestDC.MustGetK8sClient().CoreV1().(*fakecorev1.FakeCoreV1)
		core.PrependReactor("get", "secrets", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("apiserver is down")
		})
		run()
		Expect(f.ValuesGet("documentation.internal.s3Connection.S3_BUCKET").String()).To(Equal("ZG9jdW1lbnRhdGlvbg=="))
		core.ReactionChain = core.ReactionChain[1:]

		// A deleted Secret takes the copy away.
		Expect(secrets.Delete(context.TODO(), "documentation-s3", metav1.DeleteOptions{})).To(Succeed())
		run()
		Expect(f.ValuesGet("documentation.internal.s3Connection").Exists()).To(BeFalse())
	})

	It("Keeps no copy for another storage type", func() {
		f.ValuesSet("documentation.internal.s3Connection", map[string]any{"S3_BUCKET": "documentation"})
		f.ValuesSet("documentation.moduleStorage.s3.type", "System")
		run()
		Expect(f.ValuesGet("documentation.internal.s3Connection").Exists()).To(BeFalse())
	})

	It("Keeps the checksum of the ConfigMap of system-s3", func() {
		f.ValuesSet("documentation.moduleStorage.s3.type", "System")
		run()
		// No ConfigMap yet: once it appears, the checksum rolls the Deployment once.
		Expect(f.ValuesGet("documentation.internal.s3ConnectionChecksum").Exists()).To(BeFalse())

		configMaps := dependency.TestDC.MustGetK8sClient().CoreV1().ConfigMaps("d8-system")
		cm := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: "d8-system", Name: "documentation-s3-connection"},
			Data:       map[string]string{"S3_ENDPOINT": "https://s3.example.com", "S3_CA_CERT": "ca-1"},
		}
		_, err := configMaps.Create(context.TODO(), cm, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		run()
		first := f.ValuesGet("documentation.internal.s3ConnectionChecksum").String()
		Expect(first).NotTo(BeEmpty())

		// system-s3 replaced its CA: a new checksum restarts the pod.
		cm.Data["S3_CA_CERT"] = "ca-2"
		_, err = configMaps.Update(context.TODO(), cm, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())
		run()
		Expect(f.ValuesGet("documentation.internal.s3ConnectionChecksum").String()).NotTo(Equal(first))

		// Another storage type needs no checksum.
		f.ValuesSet("documentation.moduleStorage.s3.type", "Disabled")
		run()
		Expect(f.ValuesGet("documentation.internal.s3ConnectionChecksum").Exists()).To(BeFalse())
	})
})
