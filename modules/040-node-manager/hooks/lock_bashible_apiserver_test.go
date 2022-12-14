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
  input.yaml: CiAgICBjbHVzdGVyRG9tYWluOiBjbHVzdGVyLmxvY2FsCiAgICBjbHVzdGVyRE5TQWRkcmVzczogMTAuMjIyLjAuMTAKICAgIGNsb3VkUHJvdmlkZXI6CiAgICAgIGluc3RhbmNlQ2xhc3NLaW5kOiBPcGVuU3RhY2tJbnN0YW5jZUNsYXNzCiAgICAgIG1hY2hpbmVDbGFzc0tpbmQ6IE9wZW5TdGFja01hY2hpbmVDbGFzcwogICAgICBvcGVuc3RhY2s6CiAgICAgICAgY29ubmVjdGlvbjoKICAgICAgICAgIGF1dGhVUkw6IGh0dHBzOi8vY2xvdWQuZmxhbnQuY29tL3YzLwogICAgICAgICAgZG9tYWluTmFtZTogRGVmYXVsdAogICAgICAgICAgcGFzc3dvcmQ6IFdRZDlVekRFZmFWSWZCNXQKICAgICAgICAgIHJlZ2lvbjogSGV0em5lckZpbmxhbmQKICAgICAgICAgIHRlbmFudE5hbWU6IHktbG9zZXYKICAgICAgICAgIHVzZXJuYW1lOiB5LWxvc2V2CiAgICAgICAgZXh0ZXJuYWxOZXR3b3JrTmFtZXM6CiAgICAgICAgLSBwdWJsaWMKICAgICAgICBpbnN0YW5jZXM6CiAgICAgICAgICBpbWFnZU5hbWU6IHVidW50dS0xOC0wNC1jbG91ZC1hbWQ2NAogICAgICAgICAgbWFpbk5ldHdvcms6IG5kZXYKICAgICAgICAgIHNlY3VyaXR5R3JvdXBzOgogICAgICAgICAgLSBuZGV2CiAgICAgICAgICBzc2hLZXlQYWlyTmFtZTogbmRldgogICAgICAgIGludGVybmFsTmV0d29ya05hbWVzOgogICAgICAgIC0gbmRldgogICAgICAgIHBvZE5ldHdvcmtNb2RlOiBEaXJlY3RSb3V0aW5nV2l0aFBvcnRTZWN1cml0eUVuYWJsZWQKICAgICAgdHlwZTogb3BlbnN0YWNrCiAgICAgIHpvbmVzOgogICAgICAtIG5vdmEKICAgIGFwaXNlcnZlckVuZHBvaW50czoKICAgICAgLSAxOTIuMTY4LjE5OS4yMjI6NjQ0MwogICAga3ViZXJuZXRlc0NBOiB8CiAgICAgIC0tLS0tQkVHSU4gQ0VSVElGSUNBVEUtLS0tLQogICAgICBNSUlDNXpDQ0FjK2dBd0lCQWdJQkFEQU5CZ2txaGtpRzl3MEJBUXNGQURBVk1STXdFUVlEVlFRREV3cHJkV0psCiAgICAgIGNtNWxkR1Z6TUI0WERUSXlNRFl3TVRBNE1EQXlNVm9YRFRNeU1EVXlPVEE0TURBeU1Wb3dGVEVUTUJFR0ExVUUKICAgICAgQXhNS2EzVmlaWEp1WlhSbGN6Q0NBU0l3RFFZSktvWklodmNOQVFFQkJRQURnZ0VQQURDQ0FRb0NnZ0VCQU11bgogICAgICBSelhVV2JSUFErMGRiYXFsajh5TlR6NlpBWXZidTNsQmp4a05lRWFQT01pa1N1allWUW5sbUo2UStFYm9NL2dNCiAgICAgIEZPQWh1bHdQd1RBVWpWTENVTyt5am1lSzhwODloakozZTVrSEdmc3luZU9GM0tqc0svd0E0VklRREgvZWZzSTQKICAgICAgSllHVkp2WDFmMmpYZC9nUW1heHRIT25ad0xKVE9GL25MSitGN2o2czhvdG9ES3RieUxvNzZ5bW83OUliZU1ieQogICAgICB5dkNKYUltdDF5STRyWXJrUURzWW4zMGdCTTBCZmczWjNFeElBSVZEOGZzOWxxMWNhOHd3ajdsMmVrUDBoLzlVCiAgICAgIER0L3VJeU1BYWJ0TDBVa2F5UHpmZzNYcGlGRGgvNEFZY3RWMEl6cEkyZVd6amVac01CLzlMd0dVNzdDNG9jOC8KICAgICAgOUJPTHZtMWxZR083ellUc3hDVUNBd0VBQWFOQ01FQXdEZ1lEVlIwUEFRSC9CQVFEQWdLa01BOEdBMVVkRXdFQgogICAgICAvd1FGTUFNQkFmOHdIUVlEVlIwT0JCWUVGS1NOamkrMndIS3U5emFGbm9Qck95VDRCREpaTUEwR0NTcUdTSWIzCiAgICAgIERRRUJDd1VBQTRJQkFRQlBkN0w3d2Q2bDlDd3ZxR20rQ0IzR1lDd0FlWU5EM0ZRYWRua0FNbHBKemsxRHhPYWsKICAgICAgbytudXYyc0E3Y1I0cG42cnk3aDhsdktwN2F1dzdRL2ttUXBvN3JMUHRSNDh1YUU1TlVONnRGZWo5bGNPTXhrTAogICAgICBNaks4dHBENWM1Q3hwZzN2bGJETEVkQnFiUmJocDdoSWo2SG9UNlhtZWxHaWN0cm1OY1UwUExwa1JRM05CMVU3CiAgICAgIEo2VXFqeFpUdndhNlFMcURLM1J1Q3FDNEVzUUthK2UvV045NlpPWDVFVW4yaWhidW5ZeGdaMHk4dEduOFcxTmcKICAgICAgSW1kNmxCSHpSVlg4N2xQV2J2dVRUMWRBU0N3R3RJMTNlM2g4dElzbWVqTFV6SUtCVk55Q3pWSGF1dHRMWlc0ZAogICAgICBxaUNzR21Dak1uT0drQkUrYTNOUTE2ekVpekZFUmdMbmJoR0UKICAgICAgLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQogICAgICAKICAgIGFsbG93ZWRCdW5kbGVzOgogICAgICAtIHVidW50dS1sdHMKICAgICAgLSBjZW50b3MKICAgICAgLSBkZWJpYW4KICAgIGFsbG93ZWRLdWJlcm5ldGVzVmVyc2lvbnM6CiAgICAgIC0gIjEuMTkiCiAgICAgIC0gIjEuMjAiCiAgICAgIC0gIjEuMjEiCiAgICAgIC0gIjEuMjIiCiAgICAgIC0gIjEuMjMiCiAgICBub2RlR3JvdXBzOgogICAgICAtIGNyaToKICAgICAgICAgIHR5cGU6IENvbnRhaW5lcmQKICAgICAgICBkaXNydXB0aW9uczoKICAgICAgICAgIGFwcHJvdmFsTW9kZTogTWFudWFsCiAgICAgICAga3ViZXJuZXRlc1ZlcnNpb246ICIxLjIxIgogICAgICAgIG1hbnVhbFJvbGxvdXRJRDogIiIKICAgICAgICBuYW1lOiBtYXN0ZXIKICAgICAgICBub2RlVGVtcGxhdGU6CiAgICAgICAgICBsYWJlbHM6CiAgICAgICAgICAgIG5vZGUtcm9sZS5rdWJlcm5ldGVzLmlvL2NvbnRyb2wtcGxhbmU6ICIiCiAgICAgICAgICAgIG5vZGUtcm9sZS5rdWJlcm5ldGVzLmlvL21hc3RlcjogIiIKICAgICAgICAgIHRhaW50czoKICAgICAgICAgIC0gZWZmZWN0OiBOb1NjaGVkdWxlCiAgICAgICAgICAgIGtleTogbm9kZS1yb2xlLmt1YmVybmV0ZXMuaW8vbWFzdGVyCiAgICAgICAgbm9kZVR5cGU6IENsb3VkUGVybWFuZW50CiAgICAgICAgdXBkYXRlRXBvY2g6ICIxNjYwMTUwMjAyIgogICAgICAtIGNsb3VkSW5zdGFuY2VzOgogICAgICAgICAgY2xhc3NSZWZlcmVuY2U6CiAgICAgICAgICAgIGtpbmQ6IE9wZW5TdGFja0luc3RhbmNlQ2xhc3MKICAgICAgICAgICAgbmFtZTogd29ya2VyCiAgICAgICAgICBtYXhQZXJab25lOiAzCiAgICAgICAgICBtaW5QZXJab25lOiAxCiAgICAgICAgICB6b25lczoKICAgICAgICAgIC0gbm92YQogICAgICAgIGNyaToKICAgICAgICAgIHR5cGU6IENvbnRhaW5lcmQKICAgICAgICBkaXNydXB0aW9uczoKICAgICAgICAgIGFwcHJvdmFsTW9kZTogQXV0b21hdGljCiAgICAgICAgaW5zdGFuY2VDbGFzczoKICAgICAgICAgIGZsYXZvck5hbWU6IG0xLmxhcmdlCiAgICAgICAgICBpbWFnZU5hbWU6IHVidW50dS0xOC0wNC1jbG91ZC1hbWQ2NAogICAgICAgICAgbWFpbk5ldHdvcms6IG5kZXYKICAgICAgICBrdWJlcm5ldGVzVmVyc2lvbjogIjEuMjEiCiAgICAgICAgbWFudWFsUm9sbG91dElEOiAiIgogICAgICAgIG5hbWU6IHdvcmtlcgogICAgICAgIG5vZGVUeXBlOiBDbG91ZEVwaGVtZXJhbAogICAgICAgIHVwZGF0ZUVwb2NoOiAiMTY2MDE1MzM0NiIKICAgIG5vZGVTdGF0dXNVcGRhdGVGcmVxdWVuY3k6IDEw
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
  input.yaml: CiAgICBjbHVzdGVyRG9tYWluOiBjbHVzdGVyLmxvY2FsCiAgICBjbHVzdGVyRE5TQWRkcmVzczogMTAuMjIyLjAuMTAKICAgIGNsb3VkUHJvdmlkZXI6CiAgICAgIGluc3RhbmNlQ2xhc3NLaW5kOiBPcGVuU3RhY2tJbnN0YW5jZUNsYXNzCiAgICAgIG1hY2hpbmVDbGFzc0tpbmQ6IE9wZW5TdGFja01hY2hpbmVDbGFzcwogICAgICBvcGVuc3RhY2s6CiAgICAgICAgY29ubmVjdGlvbjoKICAgICAgICAgIGF1dGhVUkw6IGh0dHBzOi8vY2xvdWQuZmxhbnQuY29tL3YzLwogICAgICAgICAgZG9tYWluTmFtZTogRGVmYXVsdAogICAgICAgICAgcGFzc3dvcmQ6IFdRZDlVekRFZmFWSWZCNXQKICAgICAgICAgIHJlZ2lvbjogSGV0em5lckZpbmxhbmQKICAgICAgICAgIHRlbmFudE5hbWU6IHktbG9zZXYKICAgICAgICAgIHVzZXJuYW1lOiB5LWxvc2V2CiAgICAgICAgZXh0ZXJuYWxOZXR3b3JrTmFtZXM6CiAgICAgICAgLSBwdWJsaWMKICAgICAgICBpbnN0YW5jZXM6CiAgICAgICAgICBpbWFnZU5hbWU6IHVidW50dS0xOC0wNC1jbG91ZC1hbWQ2NAogICAgICAgICAgbWFpbk5ldHdvcms6IG5kZXYKICAgICAgICAgIHNlY3VyaXR5R3JvdXBzOgogICAgICAgICAgLSBuZGV2CiAgICAgICAgICBzc2hLZXlQYWlyTmFtZTogbmRldgogICAgICAgIGludGVybmFsTmV0d29ya05hbWVzOgogICAgICAgIC0gbmRldgogICAgICAgIHBvZE5ldHdvcmtNb2RlOiBEaXJlY3RSb3V0aW5nV2l0aFBvcnRTZWN1cml0eUVuYWJsZWQKICAgICAgdHlwZTogb3BlbnN0YWNrCiAgICAgIHpvbmVzOgogICAgICAtIG5vdmEKICAgIGFwaXNlcnZlckVuZHBvaW50czoKICAgICAgLSAxOTIuMTY4LjE5OS4yMjI6NjQ0MwogICAga3ViZXJuZXRlc0NBOiB8CiAgICAgIC0tLS0tQkVHSU4gQ0VSVElGSUNBVEUtLS0tLQogICAgICBNSUlDNXpDQ0FjK2dBd0lCQWdJQkFEQU5CZ2txaGtpRzl3MEJBUXNGQURBVk1STXdFUVlEVlFRREV3cHJkV0psCiAgICAgIGNtNWxkR1Z6TUI0WERUSXlNRFl3TVRBNE1EQXlNVm9YRFRNeU1EVXlPVEE0TURBeU1Wb3dGVEVUTUJFR0ExVUUKICAgICAgQXhNS2EzVmlaWEp1WlhSbGN6Q0NBU0l3RFFZSktvWklodmNOQVFFQkJRQURnZ0VQQURDQ0FRb0NnZ0VCQU11bgogICAgICBSelhVV2JSUFErMGRiYXFsajh5TlR6NlpBWXZidTNsQmp4a05lRWFQT01pa1N1allWUW5sbUo2UStFYm9NL2dNCiAgICAgIEZPQWh1bHdQd1RBVWpWTENVTyt5am1lSzhwODloakozZTVrSEdmc3luZU9GM0tqc0svd0E0VklRREgvZWZzSTQKICAgICAgSllHVkp2WDFmMmpYZC9nUW1heHRIT25ad0xKVE9GL25MSitGN2o2czhvdG9ES3RieUxvNzZ5bW83OUliZU1ieQogICAgICB5dkNKYUltdDF5STRyWXJrUURzWW4zMGdCTTBCZmczWjNFeElBSVZEOGZzOWxxMWNhOHd3ajdsMmVrUDBoLzlVCiAgICAgIER0L3VJeU1BYWJ0TDBVa2F5UHpmZzNYcGlGRGgvNEFZY3RWMEl6cEkyZVd6amVac01CLzlMd0dVNzdDNG9jOC8KICAgICAgOUJPTHZtMWxZR083ellUc3hDVUNBd0VBQWFOQ01FQXdEZ1lEVlIwUEFRSC9CQVFEQWdLa01BOEdBMVVkRXdFQgogICAgICAvd1FGTUFNQkFmOHdIUVlEVlIwT0JCWUVGS1NOamkrMndIS3U5emFGbm9Qck95VDRCREpaTUEwR0NTcUdTSWIzCiAgICAgIERRRUJDd1VBQTRJQkFRQlBkN0w3d2Q2bDlDd3ZxR20rQ0IzR1lDd0FlWU5EM0ZRYWRua0FNbHBKemsxRHhPYWsKICAgICAgbytudXYyc0E3Y1I0cG42cnk3aDhsdktwN2F1dzdRL2ttUXBvN3JMUHRSNDh1YUU1TlVONnRGZWo5bGNPTXhrTAogICAgICBNaks4dHBENWM1Q3hwZzN2bGJETEVkQnFiUmJocDdoSWo2SG9UNlhtZWxHaWN0cm1OY1UwUExwa1JRM05CMVU3CiAgICAgIEo2VXFqeFpUdndhNlFMcURLM1J1Q3FDNEVzUUthK2UvV045NlpPWDVFVW4yaWhidW5ZeGdaMHk4dEduOFcxTmcKICAgICAgSW1kNmxCSHpSVlg4N2xQV2J2dVRUMWRBU0N3R3RJMTNlM2g4dElzbWVqTFV6SUtCVk55Q3pWSGF1dHRMWlc0ZAogICAgICBxaUNzR21Dak1uT0drQkUrYTNOUTE2ekVpekZFUmdMbmJoR0UKICAgICAgLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQogICAgICAKICAgIGFsbG93ZWRCdW5kbGVzOgogICAgICAtIHVidW50dS1sdHMKICAgICAgLSBjZW50b3MKICAgICAgLSBkZWJpYW4KICAgIGFsbG93ZWRLdWJlcm5ldGVzVmVyc2lvbnM6CiAgICAgIC0gIjEuMTkiCiAgICAgIC0gIjEuMjAiCiAgICAgIC0gIjEuMjEiCiAgICAgIC0gIjEuMjIiCiAgICAgIC0gIjEuMjMiCiAgICBub2RlR3JvdXBzOgogICAgICAtIGNyaToKICAgICAgICAgIHR5cGU6IENvbnRhaW5lcmQKICAgICAgICBkaXNydXB0aW9uczoKICAgICAgICAgIGFwcHJvdmFsTW9kZTogTWFudWFsCiAgICAgICAga3ViZXJuZXRlc1ZlcnNpb246ICIxLjIxIgogICAgICAgIG1hbnVhbFJvbGxvdXRJRDogIiIKICAgICAgICBuYW1lOiBtYXN0ZXIKICAgICAgICBub2RlVGVtcGxhdGU6CiAgICAgICAgICBsYWJlbHM6CiAgICAgICAgICAgIG5vZGUtcm9sZS5rdWJlcm5ldGVzLmlvL2NvbnRyb2wtcGxhbmU6ICIiCiAgICAgICAgICAgIG5vZGUtcm9sZS5rdWJlcm5ldGVzLmlvL21hc3RlcjogIiIKICAgICAgICAgIHRhaW50czoKICAgICAgICAgIC0gZWZmZWN0OiBOb1NjaGVkdWxlCiAgICAgICAgICAgIGtleTogbm9kZS1yb2xlLmt1YmVybmV0ZXMuaW8vbWFzdGVyCiAgICAgICAgbm9kZVR5cGU6IENsb3VkUGVybWFuZW50CiAgICAgICAgdXBkYXRlRXBvY2g6ICIxNjYwMTUwMjAyIgogICAgICAtIGNsb3VkSW5zdGFuY2VzOgogICAgICAgICAgY2xhc3NSZWZlcmVuY2U6CiAgICAgICAgICAgIGtpbmQ6IE9wZW5TdGFja0luc3RhbmNlQ2xhc3MKICAgICAgICAgICAgbmFtZTogd29ya2VyCiAgICAgICAgICBtYXhQZXJab25lOiAzCiAgICAgICAgICBtaW5QZXJab25lOiAxCiAgICAgICAgICB6b25lczoKICAgICAgICAgIC0gbm92YQogICAgICAgIGNyaToKICAgICAgICAgIHR5cGU6IENvbnRhaW5lcmQKICAgICAgICBkaXNydXB0aW9uczoKICAgICAgICAgIGFwcHJvdmFsTW9kZTogQXV0b21hdGljCiAgICAgICAgaW5zdGFuY2VDbGFzczoKICAgICAgICAgIGZsYXZvck5hbWU6IG0xLmxhcmdlCiAgICAgICAgICBpbWFnZU5hbWU6IHVidW50dS0xOC0wNC1jbG91ZC1hbWQ2NAogICAgICAgICAgbWFpbk5ldHdvcms6IG5kZXYKICAgICAgICBrdWJlcm5ldGVzVmVyc2lvbjogIjEuMjEiCiAgICAgICAgbWFudWFsUm9sbG91dElEOiAiIgogICAgICAgIG5hbWU6IHdvcmtlcgogICAgICAgIG5vZGVUeXBlOiBDbG91ZEVwaGVtZXJhbAogICAgICAgIHVwZGF0ZUVwb2NoOiAiMTY2MDE1MzM0NiIKICAgIG5vZGVTdGF0dXNVcGRhdGVGcmVxdWVuY3k6IDEw
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
