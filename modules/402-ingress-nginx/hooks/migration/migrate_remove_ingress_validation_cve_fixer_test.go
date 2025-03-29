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
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
  name: d8-system
`

	depYaml = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: ingress-validation-cve-fixer
  name: d8-ingress-validation-cve-fixer
spec:
  replicas: 2
  selector:
    matchLabels:
      app: ingress-validation-cve-fixer
  template:
    metadata:
      labels:
        app: ingress-validation-cve-fixer
    spec:
      containers:
      - image: $shellOperatorImage
`
	cmYaml = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-ingress-validation-cve-fixer
  namespace: d8-system
data:
  disable-ingress-validation.sh: |
    #!/usr/bin/env bash
`
	svcYaml = `
---
apiVersion: v1
kind: Service
metadata:
  name: d8-ingress-validation-cve-fixer
  namespace: d8-system
spec:
  type: ClusterIP
  ports:
    - name: mutating-http
      port: 443
      targetPort: 9680
      protocol: TCP
`
	secretYaml = `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-ingress-validation-cve-fixer
  namespace: d8-system
type: kubernetes.io/tls
data:
  ca.crt: dGVzdAo=
  tls.crt: dGVzdAo=
  tls.key: dGVzdAo=
`
	crbYaml = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: d8:ingress-validation-cve-fixer
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:ingress-validation-cve-fixer
subjects:
- kind: ServiceAccount
  name: d8-ingress-validation-cve-fixer
  namespace: d8-system
`
	crYaml = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:ingress-validation-cve-fixer
rules:
- apiGroups:
  - admissionregistration.k8s.io
  resources:
  - mutatingwebhookconfigurations
  verbs:
  - create
  - list
  - update
`
	saYaml = `
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: d8-ingress-validation-cve-fixer
  namespace: d8-system
`
	mutatingwcYaml = `
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: d8-ingress-validation-cve-fixer-hooks
webhooks:
- admissionReviewVersions:
  - v1
  - v1beta1
  clientConfig:
    caBundle: dGVzdAo=
    service:
      name: d8-ingress-validation-cve-fixer
      namespace: d8-system
      path: /hooks/disable-ingress-validation
      port: 443
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: disable.ingress.validation
  namespaceSelector:
    matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values:
      - d8-ingress-nginx
  objectSelector:
    matchLabels:
      app: controller
  reinvocationPolicy: Never
  rules:
  - apiGroups:
    - ""
    apiVersions:
    - v1
    operations:
    - CREATE
    resources:
    - pods
    scope: Namespaced
  sideEffects: None
  timeoutSeconds: 10
`
)

var _ = Describe("ingress-nginx :: hooks :: remove_ingress_nginx_validation_cve_fixer_migration ::", func() {
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

	Context("Cluster with ingress-nginx validation CVE fixer installed", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext())

			createNs(f.KubeClient(), nsYaml)
			createDeployment(f.KubeClient(), depYaml)
			createCm(f.KubeClient(), cmYaml)
			createSvc(f.KubeClient(), svcYaml)
			createSecret(f.KubeClient(), secretYaml)
			createCrb(f.KubeClient(), crbYaml)
			createCr(f.KubeClient(), crYaml)
			createSa(f.KubeClient(), saYaml)
			createMutatingwc(f.KubeClient(), mutatingwcYaml)

			f.RunGoHook()
		})

		It("Fixer must be deleted ", func() {
			Expect(f).To(ExecuteSuccessfully())
			_, err := f.KubeClient().CoreV1().Namespaces().Get(context.TODO(), d8SystemNs, metav1.GetOptions{})
			Expect(err).To(BeNil())
			_, err = f.KubeClient().AppsV1().Deployments(d8SystemNs).Get(context.TODO(), fixerName, metav1.GetOptions{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
			_, err = f.KubeClient().CoreV1().ConfigMaps(d8SystemNs).Get(context.TODO(), fixerName, metav1.GetOptions{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
			_, err = f.KubeClient().CoreV1().Services(d8SystemNs).Get(context.TODO(), fixerName, metav1.GetOptions{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
			_, err = f.KubeClient().CoreV1().Secrets(d8SystemNs).Get(context.TODO(), fixerName, metav1.GetOptions{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
			_, err = f.KubeClient().RbacV1().ClusterRoleBindings().Get(context.TODO(), fixerRBACName, metav1.GetOptions{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
			_, err = f.KubeClient().RbacV1().ClusterRoles().Get(context.TODO(), fixerRBACName, metav1.GetOptions{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
			_, err = f.KubeClient().CoreV1().ServiceAccounts(d8SystemNs).Get(context.TODO(), fixerName, metav1.GetOptions{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
			_, err = f.KubeClient().AdmissionregistrationV1().MutatingWebhookConfigurations().Get(context.TODO(), fixerName+"-hooks", metav1.GetOptions{})
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

func createDeployment(kubeClient client.KubeClient, spec string) {
	dep := new(appsv1.Deployment)
	if err := yaml.Unmarshal([]byte(spec), dep); err != nil {
		panic(err)
	}
	_, _ = kubeClient.AppsV1().Deployments(d8SystemNs).Create(context.TODO(), dep, metav1.CreateOptions{})
}

func createCm(kubeClient client.KubeClient, spec string) {
	cm := new(corev1.ConfigMap)
	if err := yaml.Unmarshal([]byte(spec), cm); err != nil {
		panic(err)
	}
	_, _ = kubeClient.CoreV1().ConfigMaps(d8SystemNs).Create(context.TODO(), cm, metav1.CreateOptions{})
}

func createSvc(kubeClient client.KubeClient, spec string) {
	svc := new(corev1.Service)
	if err := yaml.Unmarshal([]byte(spec), svc); err != nil {
		panic(err)
	}
	_, _ = kubeClient.CoreV1().Services(d8SystemNs).Create(context.TODO(), svc, metav1.CreateOptions{})
}

func createSecret(kubeClient client.KubeClient, spec string) {
	secret := new(corev1.Secret)
	if err := yaml.Unmarshal([]byte(spec), secret); err != nil {
		panic(err)
	}
	_, _ = kubeClient.CoreV1().Secrets(d8SystemNs).Create(context.TODO(), secret, metav1.CreateOptions{})
}

func createCrb(kubeClient client.KubeClient, spec string) {
	crb := new(rbacv1.ClusterRoleBinding)
	if err := yaml.Unmarshal([]byte(spec), crb); err != nil {
		panic(err)
	}
	_, _ = kubeClient.RbacV1().ClusterRoleBindings().Create(context.TODO(), crb, metav1.CreateOptions{})
}

func createCr(kubeClient client.KubeClient, spec string) {
	cr := new(rbacv1.ClusterRole)
	if err := yaml.Unmarshal([]byte(spec), cr); err != nil {
		panic(err)
	}
	_, _ = kubeClient.RbacV1().ClusterRoles().Create(context.TODO(), cr, metav1.CreateOptions{})
}

func createSa(kubeClient client.KubeClient, spec string) {
	sa := new(corev1.ServiceAccount)
	if err := yaml.Unmarshal([]byte(spec), sa); err != nil {
		panic(err)
	}
	_, _ = kubeClient.CoreV1().ServiceAccounts(d8SystemNs).Create(context.TODO(), sa, metav1.CreateOptions{})
}

func createMutatingwc(kubeClient client.KubeClient, spec string) {
	mutatingwc := new(admissionregistrationv1.MutatingWebhookConfiguration)
	if err := yaml.Unmarshal([]byte(spec), mutatingwc); err != nil {
		panic(err)
	}
	_, _ = kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.TODO(), mutatingwc, metav1.CreateOptions{})
}
