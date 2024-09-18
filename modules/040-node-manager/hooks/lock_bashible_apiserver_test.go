/*
Copyright 2022 Flant JSC

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

var _ = Describe("Modules :: node-manager :: hooks :: Lock Basible Apiserver on image update ::", func() {
	f := HookExecutionConfigInit(`{"global": {"modulesImages": {"digests": {"nodeManager": {"bashibleApiserver": "sha256:8913a5815edcdebc436664ac1f654194a43df117c27b7e5ff153cdf64df30fbb"}}}}}`, `{}`)

	Context("Digests are up to date", func() {
		BeforeEach(func() {
			f.KubeStateSet(actualDeployment + bashibleSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunGoHook()
		})
		It("Should not have lock annotation", func() {
			Expect(f).To(ExecuteSuccessfully())
			serv := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-apiserver-context")
			Expect(serv.Field(`metadata.annotations.node\.deckhouse\.io\/bashible-locked`).Exists()).To(BeFalse())
		})
	})

	Context("Digests are different", func() {
		BeforeEach(func() {
			f.ValuesSet("global.modulesImages.digests.nodeManager.bashibleApiserver", "sha256:79ed551f4d0ec60799a9bd67f35441df6d86443515dd8337284fb68d97a01b3d")
			f.KubeStateSet(actualDeployment + bashibleSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunGoHook()
		})
		It("Should set lock annotation", func() {
			Expect(f).To(ExecuteSuccessfully())
			serv := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-apiserver-context")
			Expect(serv.Field(`metadata.annotations.node\.deckhouse\.io\/bashible-locked`).String()).To(Equal("true"))
		})

		Context("Deployment was updated", func() {
			BeforeEach(func() {
				f.ValuesSet("global.modulesImages.digests.nodeManager.bashibleApiserver", "sha256:79ed551f4d0ec60799a9bd67f35441df6d86443515dd8337284fb68d97a01b3d")
				f.BindingContexts.Set(f.KubeStateSet(actualDeploymentYYY + bashibleSecretLocked))
				f.RunGoHook()
			})
			It("Should remove annotation", func() {
				Expect(f).To(ExecuteSuccessfully())
				serv := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-apiserver-context")
				Expect(serv.Field(`metadata.annotations.node\.deckhouse\.io\/bashible-locked`).Exists()).To(BeFalse())
			})
		})

		Context("Deployment was updated but old pod exists", func() {
			BeforeEach(func() {
				f.ValuesSet("global.modulesImages.digests.nodeManager.bashibleApiserver", "sha256:79ed551f4d0ec60799a9bd67f35441df6d86443515dd8337284fb68d97a01b3d")
				f.BindingContexts.Set(f.KubeStateSet(outdatedDeploymentYYY + bashibleSecretLocked))
				f.RunGoHook()
			})
			It("Should keep annotation", func() {
				Expect(f).To(ExecuteSuccessfully())
				serv := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-apiserver-context")
				Expect(serv.Field(`metadata.annotations.node\.deckhouse\.io\/bashible-locked`).String()).To(Equal("true"))
			})
		})
	})

})

const (
	actualDeployment = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bashible-apiserver
  namespace: d8-cloud-instance-manager
spec:
  replicas: 1
  selector:
    matchLabels:
      app: bashible-apiserver
  template:
    metadata:
      labels:
        app: bashible-apiserver
    spec:
      containers:
      - name: bashible-apiserver
        image: dev-registry.deckhouse.io/sys/deckhouse-oss@sha256:8913a5815edcdebc436664ac1f654194a43df117c27b7e5ff153cdf64df30fbb
status:
  replicas: 2
  updatedReplicas: 2
`

	actualDeploymentYYY = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bashible-apiserver
  namespace: d8-cloud-instance-manager
spec:
  replicas: 1
  selector:
    matchLabels:
      app: bashible-apiserver
  template:
    metadata:
      labels:
        app: bashible-apiserver
    spec:
      containers:
      - name: bashible-apiserver
        image: dev-registry.deckhouse.io/sys/deckhouse-oss@sha256:79ed551f4d0ec60799a9bd67f35441df6d86443515dd8337284fb68d97a01b3d
status:
  replicas: 2
  updatedReplicas: 2
`

	outdatedDeploymentYYY = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bashible-apiserver
  namespace: d8-cloud-instance-manager
spec:
  replicas: 1
  selector:
    matchLabels:
      app: bashible-apiserver
  template:
    metadata:
      labels:
        app: bashible-apiserver
    spec:
      containers:
      - name: bashible-apiserver
        image: dev-registry.deckhouse.io/sys/deckhouse-oss@sha256:79ed551f4d0ec60799a9bd67f35441df6d86443515dd8337284fb68d97a01b3d
status:
  replicas: 2
  updatedReplicas: 1
`

	bashibleSecret = `
---
apiVersion: v1
data:
  input.yaml: dGVzdDogdGVzdA==
kind: Secret
metadata:
  labels:
    app: bashible-apiserver
  name: bashible-apiserver-context
  namespace: d8-cloud-instance-manager
type: Opaque
`

	bashibleSecretLocked = `
---
apiVersion: v1
data:
  input.yaml: dGVzdDogdGVzdA==
kind: Secret
metadata:
  annotations:
    node.deckhouse.io/bashible-locked: true
  labels:
    app: bashible-apiserver
  name: bashible-apiserver-context
  namespace: d8-cloud-instance-manager
type: Opaque
`
)
