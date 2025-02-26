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

var _ = Describe("Modules :: node-manager :: hooks :: check_config ::", func() {
	const (
		cloudGcpBad = `
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
type: Opaque
apiVersion: v1
data:
  cloud-provider-cluster-configuration.yaml: YXBpVmVyc2lvbjogZGVja2hvdXNlLmlvL3YxCmtpbmQ6IEdDUENsdXN0ZXJDb25maWd1cmF0aW9uCmxheW91dDogU3RhbmRhcmQKbWFzdGVyTm9kZUdyb3VwOgogIGFkZGl0aW9uYWxOZXR3b3JrVGFnczoKICAtIHRhZzIKICBpbnN0YW5jZUNsYXNzOgogICAgYWRkaXRpb25hbE5ldHdvcmtUYWdzOgogICAgLSB0YWcxCiAgICBkaXNhYmxlRXh0ZXJuYWxJUDogZmFsc2UKICAgIGRpc2tTaXplR2I6IDUwCiAgICBldGNkRGlza1NpemVHYjogMjAKICAgIGltYWdlOiBwcm9qZWN0cy91YnVudHUtb3MtY2xvdWQvZ2xvYmFsL2ltYWdlcy91YnVudHUtMjQwNC1ub2JsZS1hbWQ2NC12MjAyNDA1MjNhCiAgICBtYWNoaW5lVHlwZTogbjEtc3RhbmRhcmQtNAogIHJlcGxpY2FzOiAxCiAgem9uZXM6CiAgLSBldXJvcGUtd2VzdDQtYgpwcm92aWRlcjoKICByZWdpb246IGV1cm9wZS13ZXN0NAogIHNlcnZpY2VBY2NvdW50SlNPTjogfC0KICAgIHRlc3QKc3NoS2V5OiBzc2gtZWQyNTUxOSBBQUFBLi4uCnN1Ym5ldHdvcmtDSURSOiAxMC4wLjAuMC8yNAo=
`
		cloudGcp = `
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
type: Opaque
apiVersion: v1
data:
  cloud-provider-cluster-configuration.yaml: YXBpVmVyc2lvbjogZGVja2hvdXNlLmlvL3YxCmtpbmQ6IEdDUENsdXN0ZXJDb25maWd1cmF0aW9uCmxheW91dDogU3RhbmRhcmQKbWFzdGVyTm9kZUdyb3VwOgogIGluc3RhbmNlQ2xhc3M6CiAgICBhZGRpdGlvbmFsTmV0d29ya1RhZ3M6CiAgICAtIHRhZzEKICAgIGRpc2FibGVFeHRlcm5hbElQOiBmYWxzZQogICAgZGlza1NpemVHYjogNTAKICAgIGV0Y2REaXNrU2l6ZUdiOiAyMAogICAgaW1hZ2U6IHByb2plY3RzL3VidW50dS1vcy1jbG91ZC9nbG9iYWwvaW1hZ2VzL3VidW50dS0yNDA0LW5vYmxlLWFtZDY0LXYyMDI0MDUyM2EKICAgIG1hY2hpbmVUeXBlOiBuMS1zdGFuZGFyZC00CiAgcmVwbGljYXM6IDEKICB6b25lczoKICAtIGV1cm9wZS13ZXN0NC1iCnByb3ZpZGVyOgogIHJlZ2lvbjogZXVyb3BlLXdlc3Q0CiAgc2VydmljZUFjY291bnRKU09OOiB8LQogICAgdGVzdApzc2hLZXk6IHNzaC1lZDI1NTE5IEFBQUEuLi4Kc3VibmV0d29ya0NJRFI6IDEwLjAuMC4wLzI0Cg==
`
	)

	f := HookExecutionConfigInit(`{"nodeManager":{"internal": {}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("cluster GCP Bad", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(cloudGcpBad))
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
		})
	})

	Context("cluster GCP", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(cloudGcp))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
})
