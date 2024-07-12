/*
Copyright 2021 Flant JSC

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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: kube-proxy :: hooks :: remove_old_kube_proxy ::", func() {
	var state = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kubeadm:node-proxier
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kubeadm:node-proxier
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: kubeadm:kube-proxy
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kube-proxy
  namespace: kube-system
rules:
- apiGroups:
  - ""
  resourceNames:
  - kube-proxy
  resources:
  - configmaps
  verbs:
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kube-proxy
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: dkube-proxy
subjects:
- kind: ServiceAccount
  name: kube-proxy
  namespace: kube-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kube-proxy
  namespace: kube-system
secrets:
- name: kube-proxy-token-bsmhk
---
apiVersion: v1
data:
  config.conf: |
    clusterCIDR: "10.111.0.0/16"
kind: ConfigMap
metadata:
  name: kube-proxy
  namespace: kube-system
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kube-proxy
  namespace: kube-system
spec:
  selector:
    matchLabels:
      k8s-app: kube-proxy
  template:
    metadata:
      labels:
        k8s-app: kube-proxy
    spec:
      containers:
      - image: registry.deckhouse.io/deckhouse/ce/kube-proxy/kube-proxy-1-19:58231d9eb1224489c785f5a7cf3b087c5dc6cb84074f6dd653784206
        imagePullPolicy: IfNotPresent
        name: kube-proxy
      hostNetwork: true
      serviceAccount: kube-proxy
      serviceAccountName: kube-proxy
`

	f := HookExecutionConfigInit(`{"kubeProxy":{"internal": {}}}`, `{}`)
	Context("Kubeadm native resources deleted", func() {
		BeforeEach(func() {
			f.KubeStateSet(state)
			f.RunHook()
		})

		It("must be absent in cluster", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ClusterRoleBinding", "", "kubeadm:node-proxier").Exists(), BeFalse())
			Expect(f.KubernetesResource("Role", "kube-system", "kube-proxy").Exists(), BeFalse())
			Expect(f.KubernetesResource("RoleBinding", "kube-system", "kube-proxy").Exists(), BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "kube-system", "kube-proxy").Exists(), BeFalse())
			Expect(f.KubernetesResource("ConfigMap", "kube-system", "kube-proxy").Exists(), BeFalse())
			Expect(f.KubernetesResource("DaemonSet", "kube-system", "kube-proxy").Exists(), BeFalse())
		})
	})
})
