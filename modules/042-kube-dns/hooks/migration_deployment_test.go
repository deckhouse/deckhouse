// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	deploymentYAML = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: d8-kube-dns
  namespace: kube-system
spec:
  selector:
    matchLabels:
      k8s-app: kube-dns
  template:
    metadata:
      labels:
        k8s-app: kube-dns
    spec:
      containers:
      - args: []
        image: deckhouse
        imagePullPolicy: IfNotPresent
        name: coredns
        ports:
          - containerPort: 5353
            name: dns-tcp
            protocol: TCP`

	deploymentRightPorts = `
  - image: deckhouse
    imagePullPolicy: IfNotPresent
    name: coredns
    ports:
      - containerPort: 5353
        name: dns
        protocol: UDP
      - containerPort: 5353
        name: dns-tcp
        protocol: TCP
    resources: {}`
)

var _ = Describe("KubeDns hooks :: migration_deployment", func() {
	f := HookExecutionConfigInit("{}", "{}")

	Context("There are broken deployment", func() {
		BeforeEach(func() {
			f.KubeStateSet(deploymentYAML)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})
		It("Deployment has been fixed", func() {
			Expect(f).To(ExecuteSuccessfully())
			deployment := f.KubernetesResource("Deployment", "kube-system", "d8-kube-dns")

			Expect(deployment.Field("spec.template.spec.containers").String()).To(MatchYAML(deploymentRightPorts))
		})
	})
})
