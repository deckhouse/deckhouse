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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks ::  inject cabundle to sshcredentials crd ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal": {}}}`, `{}`)

	Context("Have a CRD with caBundle not injected and secret with TLS generated", func() {
		BeforeEach(func() {
			state := `
---
apiVersion: v1
data:
  ca.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJtRENDQVQ2Z0F3SUJBZ0lVQ1hSQU1hMDM0YWhCN2gxbURjWDVIMFQvYlkwd0NnWUlLb1pJemowRUF3SXcKS2pFb01DWUdBMVVFQXhNZlkyRndjeTFqYjI1MGNtOXNiR1Z5TFcxaGJtRm5aWEl0ZDJWaWFHOXZhekFlRncweQpOVEExTURZeE1EUTJNREJhRncwek5UQTFNRFF4TURRMk1EQmFNQ294S0RBbUJnTlZCQU1USDJOaGNITXRZMjl1CmRISnZiR3hsY2kxdFlXNWhaMlZ5TFhkbFltaHZiMnN3V1RBVEJnY3Foa2pPUFFJQkJnZ3Foa2pPUFFNQkJ3TkMKQUFSNjkrUjR6bkg3TWkrZ1JxK1JrM2NIV1liMVpUaUU2VTFJdE9vL3NhNnN5UmtEYTJzVEdYNzdzenJSdzJSbwo2VnhGb083UWtVN3pGRG81QWZzR3ByenpvMEl3UURBT0JnTlZIUThCQWY4RUJBTUNBUVl3RHdZRFZSMFRBUUgvCkJBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVWQ2NmYxbkJEekdmdFZLbTRTWGFsejN2OTV3TXdDZ1lJS29aSXpqMEUKQXdJRFNBQXdSUUloQUw1SjJnaCtvOUtxR2cxN1R2UjJiVHpaMmEzWDh2bE5jMGFNVzYrZlFZU1hBaUFDZXl0YwpFV3JzM3RpdTVDWXozSGtqaTQ5L3ZkSW5qWHJMRmJLNHhMVlR3QT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
kind: Secret
metadata:
  annotations:
    meta.helm.sh/release-name: node-manager
    meta.helm.sh/release-namespace: d8-system
  labels:
    app: caps-controller-manager
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: node-manager
  name: caps-controller-manager-webhook-tls
  namespace: d8-cloud-instance-manager
type: kubernetes.io/tls
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.12.0
  labels:
    heritage: deckhouse
    module: node-manager
  name: sshcredentials.deckhouse.io
spec:
  group: deckhouse.io
  names:
    kind: SSHCredentials
    listKind: SSHCredentialsList
    plural: sshcredentials
    singular: sshcredentials
  scope: Cluster
  conversion:
    strategy: Webhook
    webhook:
      clientConfig:
        service:
          namespace: d8-cloud-instance-manager
          name: caps-controller-manager-webhook-service
          path: /convert
      conversionReviewVersions:
      - v1
  versions:
    - name: v1alpha1
      schema:
        openAPIV3Schema:
          description: |
            Contains credentials required by Cluster API Provider Static (CAPS) to connect over SSH. CAPS connects to the server (virtual machine) defined in the [StaticInstance](cr.html#staticinstance) custom resource to manage its state.
            
            A reference to this resource is specified in the [credentialsRef](cr.html#staticinstance-v1alpha1-spec-credentialsref) parameter of the StaticInstance resource.
          properties:
            apiVersion:
              description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. [More info...](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)'
              type: string
            kind:
              description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. [More info...](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds)'
              type: string
            metadata:
              type: object
            spec:
              description: SSHCredentialsSpec defines the desired state of SSHCredentials.
              properties:
                privateSSHKey:
                  description: |
                    Private SSH key in PEM format encoded as base64 string.
                  type: string
                sshExtraArgs:
                  description: |
                    A list of additional arguments to pass to the openssh command.
                  type: string
                  x-doc-examples:
                    - -vvv
                    - -c chacha20-poly1305@openssh.com
                    - -c aes256-gcm@openssh.com
                    - -m umac-64-etm@openssh.com
                    - -m hmac-sha2-512-etm@openssh.com
                sshPort:
                  description: |
                    A port to connect to the host via SSH.
                  default: 22
                  maximum: 65535
                  minimum: 1
                  type: integer
                sudoPassword:
                  description: |
                    A sudo password for the user.
                  type: string
                user:
                  description: |
                    A username to connect to the host via SSH.
                  type: string
              required:
                - privateSSHKey
                - user
              type: object
          type: object
      served: true
      storage: false
    - name: v1alpha2
      schema:
        openAPIV3Schema:
          description: |
            Contains credentials required by Cluster API Provider Static (CAPS) to connect over SSH. CAPS connects to the server (virtual machine) defined in the [StaticInstance](cr.html#staticinstance) custom resource to manage its state.
            
            A reference to this resource is specified in the [credentialsRef](cr.html#staticinstance-v1alpha1-spec-credentialsref) parameter of the StaticInstance resource.
          properties:
            apiVersion:
              description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. [More info...](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)'
              type: string
            kind:
              description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. [More info...](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds)'
              type: string
            metadata:
              type: object
            spec:
              description: SSHCredentialsSpec defines the desired state of SSHCredentials.
              properties:
                privateSSHKey:
                  description: |
                    Private SSH key in PEM format encoded as base64 string.
                  type: string
                sshExtraArgs:
                  description: |
                    A list of additional arguments to pass to the openssh command.
                  type: string
                  x-doc-examples:
                    - -vvv
                    - -c chacha20-poly1305@openssh.com
                    - -c aes256-gcm@openssh.com
                    - -m umac-64-etm@openssh.com
                    - -m hmac-sha2-512-etm@openssh.com
                sshPort:
                  description: |
                    A port to connect to the host via SSH.
                  default: 22
                  maximum: 65535
                  minimum: 1
                  type: integer
                sudoPassword:
                  description: |
                    A sudo password for the user.
                  type: string
                  x-doc-deprecated: true
                sudoPasswordEncoded:
                  description: |
                    Base64 encoded sudo password for the user.
                  type: string
                user:
                  description: |
                    A username to connect to the host via SSH.
                  type: string
              required:
                - privateSSHKey
                - user
              type: object
          type: object
      served: true
      storage: true
`
			f.KubeStateSet(state)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})
		It("Should be a CRD with caBundle injected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("CustomResourceDefinition", "", "sshcredentials.deckhouse.io").Field(`spec.conversion.webhook.clientConfig.caBundle`).Exists()).To(BeFalse())
		})
	})

	Context("Have a CRD with caBundle injecte, service and secret with TLS generated", func() {
		BeforeEach(func() {
			state := `
---
apiVersion: v1
kind: Service
metadata:
  name: caps-controller-manager-webhook-service
  namespace: d8-cloud-instance-manager
spec:
  ports:
    - port: 443
      targetPort: webhook-server
  selector:
    app: caps-controller-manager
---
apiVersion: v1
data:
  ca.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJtRENDQVQ2Z0F3SUJBZ0lVQ1hSQU1hMDM0YWhCN2gxbURjWDVIMFQvYlkwd0NnWUlLb1pJemowRUF3SXcKS2pFb01DWUdBMVVFQXhNZlkyRndjeTFqYjI1MGNtOXNiR1Z5TFcxaGJtRm5aWEl0ZDJWaWFHOXZhekFlRncweQpOVEExTURZeE1EUTJNREJhRncwek5UQTFNRFF4TURRMk1EQmFNQ294S0RBbUJnTlZCQU1USDJOaGNITXRZMjl1CmRISnZiR3hsY2kxdFlXNWhaMlZ5TFhkbFltaHZiMnN3V1RBVEJnY3Foa2pPUFFJQkJnZ3Foa2pPUFFNQkJ3TkMKQUFSNjkrUjR6bkg3TWkrZ1JxK1JrM2NIV1liMVpUaUU2VTFJdE9vL3NhNnN5UmtEYTJzVEdYNzdzenJSdzJSbwo2VnhGb083UWtVN3pGRG81QWZzR3ByenpvMEl3UURBT0JnTlZIUThCQWY4RUJBTUNBUVl3RHdZRFZSMFRBUUgvCkJBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVWQ2NmYxbkJEekdmdFZLbTRTWGFsejN2OTV3TXdDZ1lJS29aSXpqMEUKQXdJRFNBQXdSUUloQUw1SjJnaCtvOUtxR2cxN1R2UjJiVHpaMmEzWDh2bE5jMGFNVzYrZlFZU1hBaUFDZXl0YwpFV3JzM3RpdTVDWXozSGtqaTQ5L3ZkSW5qWHJMRmJLNHhMVlR3QT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
kind: Secret
metadata:
  annotations:
    meta.helm.sh/release-name: node-manager
    meta.helm.sh/release-namespace: d8-system
  labels:
    app: caps-controller-manager
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: node-manager
  name: caps-controller-manager-webhook-tls
  namespace: d8-cloud-instance-manager
type: kubernetes.io/tls
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.12.0
  labels:
    heritage: deckhouse
    module: node-manager
  name: sshcredentials.deckhouse.io
spec:
  group: deckhouse.io
  names:
    kind: SSHCredentials
    listKind: SSHCredentialsList
    plural: sshcredentials
    singular: sshcredentials
  scope: Cluster
  conversion:
    strategy: Webhook
    webhook:
      clientConfig:
        caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJtRENDQVQ2Z0F3SUJBZ0lVQ1hSQU1hMDM0YWhCN2gxbURjWDVIMFQvYlkwd0NnWUlLb1pJemowRUF3SXcKS2pFb01DWUdBMVVFQXhNZlkyRndjeTFqYjI1MGNtOXNiR1Z5TFcxaGJtRm5aWEl0ZDJWaWFHOXZhekFlRncweQpOVEExTURZeE1EUTJNREJhRncwek5UQTFNRFF4TURRMk1EQmFNQ294S0RBbUJnTlZCQU1USDJOaGNITXRZMjl1CmRISnZiR3hsY2kxdFlXNWhaMlZ5TFhkbFltaHZiMnN3V1RBVEJnY3Foa2pPUFFJQkJnZ3Foa2pPUFFNQkJ3TkMKQUFSNjkrUjR6bkg3TWkrZ1JxK1JrM2NIV1liMVpUaUU2VTFJdE9vL3NhNnN5UmtEYTJzVEdYNzdzenJSdzJSbwo2VnhGb083UWtVN3pGRG81QWZzR3ByenpvMEl3UURBT0JnTlZIUThCQWY4RUJBTUNBUVl3RHdZRFZSMFRBUUgvCkJBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVWQ2NmYxbkJEekdmdFZLbTRTWGFsejN2OTV3TXdDZ1lJS29aSXpqMEUKQXdJRFNBQXdSUUloQUw1SjJnaCtvOUtxR2cxN1R2UjJiVHpaMmEzWDh2bE5jMGFNVzYrZlFZU1hBaUFDZXl0YwpFV3JzM3RpdTVDWXozSGtqaTQ5L3ZkSW5qWHJMRmJLNHhMVlR3QT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
        service:
          namespace: d8-cloud-instance-manager
          name: caps-controller-manager-webhook-service
          path: /convert
      conversionReviewVersions:
      - v1
  versions:
    - name: v1alpha1
      schema:
        openAPIV3Schema:
          description: |
            Contains credentials required by Cluster API Provider Static (CAPS) to connect over SSH. CAPS connects to the server (virtual machine) defined in the [StaticInstance](cr.html#staticinstance) custom resource to manage its state.
            
            A reference to this resource is specified in the [credentialsRef](cr.html#staticinstance-v1alpha1-spec-credentialsref) parameter of the StaticInstance resource.
          properties:
            apiVersion:
              description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. [More info...](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)'
              type: string
            kind:
              description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. [More info...](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds)'
              type: string
            metadata:
              type: object
            spec:
              description: SSHCredentialsSpec defines the desired state of SSHCredentials.
              properties:
                privateSSHKey:
                  description: |
                    Private SSH key in PEM format encoded as base64 string.
                  type: string
                sshExtraArgs:
                  description: |
                    A list of additional arguments to pass to the openssh command.
                  type: string
                  x-doc-examples:
                    - -vvv
                    - -c chacha20-poly1305@openssh.com
                    - -c aes256-gcm@openssh.com
                    - -m umac-64-etm@openssh.com
                    - -m hmac-sha2-512-etm@openssh.com
                sshPort:
                  description: |
                    A port to connect to the host via SSH.
                  default: 22
                  maximum: 65535
                  minimum: 1
                  type: integer
                sudoPassword:
                  description: |
                    A sudo password for the user.
                  type: string
                user:
                  description: |
                    A username to connect to the host via SSH.
                  type: string
              required:
                - privateSSHKey
                - user
              type: object
          type: object
      served: true
      storage: false
    - name: v1alpha2
      schema:
        openAPIV3Schema:
          description: |
            Contains credentials required by Cluster API Provider Static (CAPS) to connect over SSH. CAPS connects to the server (virtual machine) defined in the [StaticInstance](cr.html#staticinstance) custom resource to manage its state.
            
            A reference to this resource is specified in the [credentialsRef](cr.html#staticinstance-v1alpha1-spec-credentialsref) parameter of the StaticInstance resource.
          properties:
            apiVersion:
              description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. [More info...](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)'
              type: string
            kind:
              description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. [More info...](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds)'
              type: string
            metadata:
              type: object
            spec:
              description: SSHCredentialsSpec defines the desired state of SSHCredentials.
              properties:
                privateSSHKey:
                  description: |
                    Private SSH key in PEM format encoded as base64 string.
                  type: string
                sshExtraArgs:
                  description: |
                    A list of additional arguments to pass to the openssh command.
                  type: string
                  x-doc-examples:
                    - -vvv
                    - -c chacha20-poly1305@openssh.com
                    - -c aes256-gcm@openssh.com
                    - -m umac-64-etm@openssh.com
                    - -m hmac-sha2-512-etm@openssh.com
                sshPort:
                  description: |
                    A port to connect to the host via SSH.
                  default: 22
                  maximum: 65535
                  minimum: 1
                  type: integer
                sudoPassword:
                  description: |
                    A sudo password for the user.
                  type: string
                  x-doc-deprecated: true
                sudoPasswordEncoded:
                  description: |
                    Base64 encoded sudo password for the user.
                  type: string
                user:
                  description: |
                    A username to connect to the host via SSH.
                  type: string
              required:
                - privateSSHKey
                - user
              type: object
          type: object
      served: true
      storage: true
`
			f.KubeStateSet(state)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})
		It("Should be a CRD with caBundle injected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("CustomResourceDefinition", "", "sshcredentials.deckhouse.io").Field(`spec.conversion.webhook.clientConfig.caBundle`).Exists()).To(BeTrue())
			Expect(f.KubernetesResource("CustomResourceDefinition", "", "sshcredentials.deckhouse.io").Field(`spec.conversion.webhook.clientConfig.caBundle`).String()).To(Equal("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJtRENDQVQ2Z0F3SUJBZ0lVQ1hSQU1hMDM0YWhCN2gxbURjWDVIMFQvYlkwd0NnWUlLb1pJemowRUF3SXcKS2pFb01DWUdBMVVFQXhNZlkyRndjeTFqYjI1MGNtOXNiR1Z5TFcxaGJtRm5aWEl0ZDJWaWFHOXZhekFlRncweQpOVEExTURZeE1EUTJNREJhRncwek5UQTFNRFF4TURRMk1EQmFNQ294S0RBbUJnTlZCQU1USDJOaGNITXRZMjl1CmRISnZiR3hsY2kxdFlXNWhaMlZ5TFhkbFltaHZiMnN3V1RBVEJnY3Foa2pPUFFJQkJnZ3Foa2pPUFFNQkJ3TkMKQUFSNjkrUjR6bkg3TWkrZ1JxK1JrM2NIV1liMVpUaUU2VTFJdE9vL3NhNnN5UmtEYTJzVEdYNzdzenJSdzJSbwo2VnhGb083UWtVN3pGRG81QWZzR3ByenpvMEl3UURBT0JnTlZIUThCQWY4RUJBTUNBUVl3RHdZRFZSMFRBUUgvCkJBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVWQ2NmYxbkJEekdmdFZLbTRTWGFsejN2OTV3TXdDZ1lJS29aSXpqMEUKQXdJRFNBQXdSUUloQUw1SjJnaCtvOUtxR2cxN1R2UjJiVHpaMmEzWDh2bE5jMGFNVzYrZlFZU1hBaUFDZXl0YwpFV3JzM3RpdTVDWXozSGtqaTQ5L3ZkSW5qWHJMRmJLNHhMVlR3QT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K"))
		})
	})

	Context("Have a CRD with no spec.conversion set and secret with TLS generated", func() {
		BeforeEach(func() {
			stateNew := `
---
apiVersion: v1
kind: Service
metadata:
  name: caps-controller-manager-webhook-service
  namespace: d8-cloud-instance-manager
spec:
  ports:
    - port: 443
      targetPort: webhook-server
  selector:
    app: caps-controller-manager
---
apiVersion: v1
data:
  ca.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJtRENDQVQ2Z0F3SUJBZ0lVQ1hSQU1hMDM0YWhCN2gxbURjWDVIMFQvYlkwd0NnWUlLb1pJemowRUF3SXcKS2pFb01DWUdBMVVFQXhNZlkyRndjeTFqYjI1MGNtOXNiR1Z5TFcxaGJtRm5aWEl0ZDJWaWFHOXZhekFlRncweQpOVEExTURZeE1EUTJNREJhRncwek5UQTFNRFF4TURRMk1EQmFNQ294S0RBbUJnTlZCQU1USDJOaGNITXRZMjl1CmRISnZiR3hsY2kxdFlXNWhaMlZ5TFhkbFltaHZiMnN3V1RBVEJnY3Foa2pPUFFJQkJnZ3Foa2pPUFFNQkJ3TkMKQUFSNjkrUjR6bkg3TWkrZ1JxK1JrM2NIV1liMVpUaUU2VTFJdE9vL3NhNnN5UmtEYTJzVEdYNzdzenJSdzJSbwo2VnhGb083UWtVN3pGRG81QWZzR3ByenpvMEl3UURBT0JnTlZIUThCQWY4RUJBTUNBUVl3RHdZRFZSMFRBUUgvCkJBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVWQ2NmYxbkJEekdmdFZLbTRTWGFsejN2OTV3TXdDZ1lJS29aSXpqMEUKQXdJRFNBQXdSUUloQUw1SjJnaCtvOUtxR2cxN1R2UjJiVHpaMmEzWDh2bE5jMGFNVzYrZlFZU1hBaUFDZXl0YwpFV3JzM3RpdTVDWXozSGtqaTQ5L3ZkSW5qWHJMRmJLNHhMVlR3QT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
kind: Secret
metadata:
  annotations:
    meta.helm.sh/release-name: node-manager
    meta.helm.sh/release-namespace: d8-system
  labels:
    app: caps-controller-manager
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: node-manager
  name: caps-controller-manager-webhook-tls
  namespace: d8-cloud-instance-manager
type: kubernetes.io/tls
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.12.0
  labels:
    heritage: deckhouse
    module: node-manager
  name: sshcredentials.deckhouse.io
spec:
  group: deckhouse.io
  names:
    kind: SSHCredentials
    listKind: SSHCredentialsList
    plural: sshcredentials
    singular: sshcredentials
  scope: Cluster
  versions:
    - name: v1alpha1
      schema:
        openAPIV3Schema:
          description: |
            Contains credentials required by Cluster API Provider Static (CAPS) to connect over SSH. CAPS connects to the server (virtual machine) defined in the [StaticInstance](cr.html#staticinstance) custom resource to manage its state.
            
            A reference to this resource is specified in the [credentialsRef](cr.html#staticinstance-v1alpha1-spec-credentialsref) parameter of the StaticInstance resource.
          properties:
            apiVersion:
              description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. [More info...](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)'
              type: string
            kind:
              description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. [More info...](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds)'
              type: string
            metadata:
              type: object
            spec:
              description: SSHCredentialsSpec defines the desired state of SSHCredentials.
              properties:
                privateSSHKey:
                  description: |
                    Private SSH key in PEM format encoded as base64 string.
                  type: string
                sshExtraArgs:
                  description: |
                    A list of additional arguments to pass to the openssh command.
                  type: string
                  x-doc-examples:
                    - -vvv
                    - -c chacha20-poly1305@openssh.com
                    - -c aes256-gcm@openssh.com
                    - -m umac-64-etm@openssh.com
                    - -m hmac-sha2-512-etm@openssh.com
                sshPort:
                  description: |
                    A port to connect to the host via SSH.
                  default: 22
                  maximum: 65535
                  minimum: 1
                  type: integer
                sudoPassword:
                  description: |
                    A sudo password for the user.
                  type: string
                user:
                  description: |
                    A username to connect to the host via SSH.
                  type: string
              required:
                - privateSSHKey
                - user
              type: object
          type: object
      served: true
      storage: false
    - name: v1alpha2
      schema:
        openAPIV3Schema:
          description: |
            Contains credentials required by Cluster API Provider Static (CAPS) to connect over SSH. CAPS connects to the server (virtual machine) defined in the [StaticInstance](cr.html#staticinstance) custom resource to manage its state.
            
            A reference to this resource is specified in the [credentialsRef](cr.html#staticinstance-v1alpha1-spec-credentialsref) parameter of the StaticInstance resource.
          properties:
            apiVersion:
              description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. [More info...](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)'
              type: string
            kind:
              description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. [More info...](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds)'
              type: string
            metadata:
              type: object
            spec:
              description: SSHCredentialsSpec defines the desired state of SSHCredentials.
              properties:
                privateSSHKey:
                  description: |
                    Private SSH key in PEM format encoded as base64 string.
                  type: string
                sshExtraArgs:
                  description: |
                    A list of additional arguments to pass to the openssh command.
                  type: string
                  x-doc-examples:
                    - -vvv
                    - -c chacha20-poly1305@openssh.com
                    - -c aes256-gcm@openssh.com
                    - -m umac-64-etm@openssh.com
                    - -m hmac-sha2-512-etm@openssh.com
                sshPort:
                  description: |
                    A port to connect to the host via SSH.
                  default: 22
                  maximum: 65535
                  minimum: 1
                  type: integer
                sudoPassword:
                  description: |
                    A sudo password for the user.
                  type: string
                  x-doc-deprecated: true
                sudoPasswordEncoded:
                  description: |
                    Base64 encoded sudo password for the user.
                  type: string
                user:
                  description: |
                    A username to connect to the host via SSH.
                  type: string
              required:
                - privateSSHKey
                - user
              type: object
          type: object
      served: true
      storage: true
`
			f.KubeStateSet(stateNew)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})
		It("Should be a CRD with caBundle injected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("CustomResourceDefinition", "", "sshcredentials.deckhouse.io").Field(`spec.conversion.webhook.clientConfig.caBundle`).Exists()).To(BeTrue())
		})
	})

	Context("Have no CRD and secret with TLS generated", func() {
		BeforeEach(func() {
			stateNoCRD := `
---
apiVersion: v1
data:
  ca.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJtRENDQVQ2Z0F3SUJBZ0lVQ1hSQU1hMDM0YWhCN2gxbURjWDVIMFQvYlkwd0NnWUlLb1pJemowRUF3SXcKS2pFb01DWUdBMVVFQXhNZlkyRndjeTFqYjI1MGNtOXNiR1Z5TFcxaGJtRm5aWEl0ZDJWaWFHOXZhekFlRncweQpOVEExTURZeE1EUTJNREJhRncwek5UQTFNRFF4TURRMk1EQmFNQ294S0RBbUJnTlZCQU1USDJOaGNITXRZMjl1CmRISnZiR3hsY2kxdFlXNWhaMlZ5TFhkbFltaHZiMnN3V1RBVEJnY3Foa2pPUFFJQkJnZ3Foa2pPUFFNQkJ3TkMKQUFSNjkrUjR6bkg3TWkrZ1JxK1JrM2NIV1liMVpUaUU2VTFJdE9vL3NhNnN5UmtEYTJzVEdYNzdzenJSdzJSbwo2VnhGb083UWtVN3pGRG81QWZzR3ByenpvMEl3UURBT0JnTlZIUThCQWY4RUJBTUNBUVl3RHdZRFZSMFRBUUgvCkJBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVWQ2NmYxbkJEekdmdFZLbTRTWGFsejN2OTV3TXdDZ1lJS29aSXpqMEUKQXdJRFNBQXdSUUloQUw1SjJnaCtvOUtxR2cxN1R2UjJiVHpaMmEzWDh2bE5jMGFNVzYrZlFZU1hBaUFDZXl0YwpFV3JzM3RpdTVDWXozSGtqaTQ5L3ZkSW5qWHJMRmJLNHhMVlR3QT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
kind: Secret
metadata:
  annotations:
    meta.helm.sh/release-name: node-manager
    meta.helm.sh/release-namespace: d8-system
  labels:
    app: caps-controller-manager
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: node-manager
  name: caps-controller-manager-webhook-tls
  namespace: d8-cloud-instance-manager
type: kubernetes.io/tls

`
			f.KubeStateSet(stateNoCRD)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

})
