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
	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

var _ = Describe("openvpn :: hooks :: check_server_cert_expiry ::", func() {

	const (
		d8OpenvpnNamespace = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-openvpn
  labels:
    heritage: deckhouse
    module: openvpn
spec:
  finalizers:
  - kubernetes
`
		emptySecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: openvpn-pki-server
  namespace: d8-openvpn
type: Opaque
data: {}
`
		invalidCertSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: openvpn-pki-server
  namespace: d8-openvpn
type: Opaque
data:
  tls.crt: aW52YWxpZCBkYXRhCg==
`
		expiredCertSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: openvpn-pki-server
  namespace: d8-openvpn
type: Opaque
data:
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURDRENDQWZDZ0F3SUJBZ0lCQVRBTkJna3Foa2lHOXcwQkFRc0ZBREFvTVJJd0VBWURWUVFLRXdsR2JHRnUKZENCS1UwTXhFakFRQmdOVkJBTVRDV1pzWVc1MExtTnZiVEFlRncweU5EQTBNRE14TmpJNE5EaGFGdzB5TkRBMApNREl4TmpJNE5EaGFNQ2d4RWpBUUJnTlZCQW9UQ1Vac1lXNTBJRXBUUXpFU01CQUdBMVVFQXhNSlpteGhiblF1ClkyOXRNSUlCSWpBTkJna3Foa2lHOXcwQkFRRUZBQU9DQVE4QU1JSUJDZ0tDQVFFQTAzcWNEaEtVZGFHWnp2SGcKNFk2ZkVtclMxN0NMKzl1QWdnWDdlbFJLWXZ6Q3pZTXlNbmNhR08zTGs5cUxJVjZOS0JGTDcrd01qYklnSjV5bwpvcCtZVTVwalFkU3owWnVvRVNyWDd4S05GWnh3cVJZME5KTmtoaTRkVERxWnZ1R1JCeTZVbDVaMFNCSjliRFQzCjVHdkhYMjFtTHJDdmVoZDRBYTZQU05VQXFweG85VGw3elZRS3J5Y2NQTUtvdEE0ZlZ0VkFvOHVkSXZIYVphZFUKMnZZSEFUazc3TGMrRHNjUi9YL2lYcUVMdkozR1VkWGxvNXFpWVZwN0pXZzY2RXRrNm5HWnhaZ2sxNEJHaWw3RQp4bEM1WkJyUWFPTFdwOS84S2ppT0U2MEFKZXdmdXpZdklTQ3RSZVZxSzEwUzExQTd6bUtvOGZvdUgxVGRteDlWCkNrM2Myd0lEQVFBQm96MHdPekFPQmdOVkhROEJBZjhFQkFNQ0JhQXdFd1lEVlIwbEJBd3dDZ1lJS3dZQkJRVUgKQXdFd0ZBWURWUjBSQkEwd0M0SUpabXhoYm5RdVkyOXRNQTBHQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUI1aW5EcQpYZjR0Nk1rOVQrZHFJR0QwR1pEOE5WNHlOeU9wSmFQaUhIZ0JGVmZMcW1QTlc0aVlQVHVuazc0OVFMOW14dEV6CnRMN1o2bUp0Wnk2Q1NEcEpHRE9pYk5nQ29iaGxqUkhsUkp6S0lZWUxnSHR0a3lYSFNOV1dUeXMzWS81L3ZRTWcKUkxEUmlyUzU4TytvcVp3WTlGZm1lbDNRSU9vVXpBRzU1c2IxRlhLL3Z2MDNMWlhtWnkzZ3d1UEJjbGcrQ0I3eAoweVUyZEF5TmhwM09Jd0hSUWtRUFdHVndUS0IwazFwOHFYcndFdmtnT3FMZ3dhYTdhMThKeTRxZTBkaFpJUFBSCmtvNlNGTksvOWdqY21hZ1BFTGhDTm5kL1pWS1ZhTGtQTzNob2QyOWVUbGg5bkMrbzNRSGlndis5dnc1S3d6WWUKUFlkUnhOZUJILytMcnlpQwotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCgo=
`
		validCertSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: openvpn-pki-server
  namespace: d8-openvpn
type: Opaque
data:
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURDekNDQWZPZ0F3SUJBZ0lVTlJLbXJNVEE2bmdUdmtQTGJBUG54M2RLR0tJd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0ZERVNNQkFHQTFVRUF3d0pabXhoYm5RdVkyOXRNQ0FYRFRJMU1EUXdNekUwTURJek9Wb1lEekl4TWpVdwpNekV3TVRRd01qTTVXakFVTVJJd0VBWURWUVFEREFsbWJHRnVkQzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDNFkzRURERmNWZkhmUXJKcG5NejBRWTRuazVmSFM5OG9XZUpmZUNkSlQKTk9OUTVGd1hCSkM1clBrYXZTRnZIWWprNmEveHROTXM1eVc0bm9wS1VnR1BZU2dUNEd4TWdRZW5YV2F1NmhvVApuT2NsakhVR2o3TlVSS1FKL3lPY1ZKVDl0OW5pYS9ZNHVoOHMxK2VHQ0tOOWZTeDBVbHBEZWtkVTNWQjRuR2VGCmU4VUR0TGlWNm1CbTRndzArUmwrVkQxUkVJYU5iWExHcVMxWUtIcWcvZGhpRHpHVUxRakdhOFZyTU1TbkRHL00KRW8yVjA5VHN2TXh4b04waE9MZUROTjlUZVl6TnVERFRWRkJhT3U0Nk11NmRmUVdLb29leXNET1FpdzRqeGptMgpBU1JqUXdPbisrakxJWlBEUmI0UEhZZ0xBcnlkdkpXNFpUTzErVmh0aDg2WkFnTUJBQUdqVXpCUk1CMEdBMVVkCkRnUVdCQlNONUdYNi8wNE1ITEZUa0hzUmxTZWovMWxKR2pBZkJnTlZIU01FR0RBV2dCU041R1g2LzA0TUhMRlQKa0hzUmxTZWovMWxKR2pBUEJnTlZIUk1CQWY4RUJUQURBUUgvTUEwR0NTcUdTSWIzRFFFQkN3VUFBNElCQVFCOApLd1FmcXQybENMY0xpRUlEZE9hWmNMZ2tRcGZLKzN6aS90bTdDaUNyL0oxSGZkZTZYS29ONTFKQVcwQ1RZdmQyCnNDWWtkTUx6TkR3QlZTZ0lxd3luc3NRMzZTSzFyQWl6YnF2cjZWNUYxd1cwWVMraEhUSWM2cDA3TUpUMFVUdXkKVUg5Vkx5RWR5UnNETDBIbTNCcXR1YU8wcjlGVWw0N3VJVENMa0xVYXI0cTJtYlBFM1YwcFNzWlBXM2tlL0NvWQpEU2U2TUtpVFlHbnBDU3Q1NkY0ZHlnVTBMMVU4SWZsS3dnamx1NEhsazJIRHpMNndoelhCWlVycEFqM3c4dEF2CjJmN2xFZ1RhdDVpV1lNTi9iVDA2R1NsR1hHS0ExTmpQM3g2KzkzV1QzWjdSZzM0ZlRYSDNibFZNYUEzd0hnamwKQjkxeEREMVFWbzVsQkVnU0g0K3AKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
`

		openvpnStatefulSet = `
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: openvpn
  namespace: d8-openvpn
spec:
  serviceName: openvpn
  replicas: 1
  selector:
    matchLabels:
      app: openvpn
  template:
    metadata:
      labels:
        app: openvpn
    spec:
      containers:
      - name: openvpn
        image: openvpn:latest
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("An empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.KubeStateSet(``)
			f.RunGoHook()
		})

		It("Hook is executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with expired cert", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 */6 * * *"))

			var ns corev1.Namespace
			var secret corev1.Secret
			var statefulSet appsv1.StatefulSet

			_ = yaml.Unmarshal([]byte(d8OpenvpnNamespace), &ns)
			_ = yaml.Unmarshal([]byte(expiredCertSecret), &secret)
			_ = yaml.Unmarshal([]byte(openvpnStatefulSet), &statefulSet)

			// Create resources
			k8sClient := f.BindingContextController.FakeCluster().Client
			_, err := k8sClient.CoreV1().Namespaces().Create(context.TODO(), &ns, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			_, err = k8sClient.CoreV1().Secrets("d8-openvpn").Create(context.TODO(), &secret, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			_, err = k8sClient.AppsV1().StatefulSets("d8-openvpn").Create(context.TODO(), &statefulSet, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.RunGoHook()

		})

		// Check cert is deleted
		It("Hook is executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			k8sClient := f.BindingContextController.FakeCluster().Client
			_, err := k8sClient.CoreV1().Namespaces().Get(context.Background(), "openvpn-pki-server", metav1.GetOptions{})
			Expect(errors.IsNotFound(err)).To(BeTrue(), "Secret 'openvpn-pki-server' should be deleted")
		})
	})

	Context("Cluster with valid cert", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 */6 * * *"))

			var ns corev1.Namespace
			var secret corev1.Secret
			var statefulSet appsv1.StatefulSet

			_ = yaml.Unmarshal([]byte(d8OpenvpnNamespace), &ns)
			_ = yaml.Unmarshal([]byte(validCertSecret), &secret)
			_ = yaml.Unmarshal([]byte(openvpnStatefulSet), &statefulSet)

			// Create resources
			k8sClient := f.BindingContextController.FakeCluster().Client
			_, err := k8sClient.CoreV1().Namespaces().Create(context.TODO(), &ns, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			_, err = k8sClient.CoreV1().Secrets("d8-openvpn").Create(context.TODO(), &secret, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			_, err = k8sClient.AppsV1().StatefulSets("d8-openvpn").Create(context.TODO(), &statefulSet, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.RunGoHook()

		})

		// Check cert is not deleted
		It("Hook is executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			k8sClient := f.BindingContextController.FakeCluster().Client
			_, err := k8sClient.CoreV1().Secrets("d8-openvpn").Get(context.Background(), "openvpn-pki-server", metav1.GetOptions{})
			Expect(err).To(BeNil(), "Secret 'openvpn-pki-server' should still exist")
		})
	})

	Context("Cluster with empty secret (no cert data)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 */6 * * *"))

			var ns corev1.Namespace
			var secret corev1.Secret
			var statefulSet appsv1.StatefulSet

			_ = yaml.Unmarshal([]byte(d8OpenvpnNamespace), &ns)
			_ = yaml.Unmarshal([]byte(emptySecret), &secret)
			_ = yaml.Unmarshal([]byte(openvpnStatefulSet), &statefulSet)

			// Create resources
			k8sClient := f.BindingContextController.FakeCluster().Client
			_, err := k8sClient.CoreV1().Namespaces().Create(context.TODO(), &ns, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			_, err = k8sClient.CoreV1().Secrets("d8-openvpn").Create(context.TODO(), &secret, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			_, err = k8sClient.AppsV1().StatefulSets("d8-openvpn").Create(context.TODO(), &statefulSet, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.RunGoHook()
		})

		// Check secret is not deleted
		It("Hook does not delete empty secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			k8sClient := f.BindingContextController.FakeCluster().Client
			_, err := k8sClient.CoreV1().Secrets("d8-openvpn").Get(context.Background(), "openvpn-pki-server", metav1.GetOptions{})
			Expect(err).To(BeNil(), "Secret 'openvpn-pki-server' should still exist")
		})
	})

	Context("Cluster with invalid cert", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 */6 * * *"))

			var ns corev1.Namespace
			var secret corev1.Secret
			var statefulSet appsv1.StatefulSet

			_ = yaml.Unmarshal([]byte(d8OpenvpnNamespace), &ns)
			_ = yaml.Unmarshal([]byte(invalidCertSecret), &secret)
			_ = yaml.Unmarshal([]byte(openvpnStatefulSet), &statefulSet)

			// Create resources
			k8sClient := f.BindingContextController.FakeCluster().Client
			_, err := k8sClient.CoreV1().Namespaces().Create(context.TODO(), &ns, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			_, err = k8sClient.CoreV1().Secrets("d8-openvpn").Create(context.TODO(), &secret, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			_, err = k8sClient.AppsV1().StatefulSets("d8-openvpn").Create(context.TODO(), &statefulSet, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.RunGoHook()
		})

		// Check secret is not deleted
		It("Hook does not delete invalid certificate secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			k8sClient := f.BindingContextController.FakeCluster().Client
			_, err := k8sClient.CoreV1().Secrets("d8-openvpn").Get(context.Background(), "openvpn-pki-server", metav1.GetOptions{})
			Expect(err).To(BeNil(), "Secret 'openvpn-pki-server' should still exist")
		})
	})
})
