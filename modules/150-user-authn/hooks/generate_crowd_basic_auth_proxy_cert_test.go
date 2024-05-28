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
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/helm"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: generate crowd auth proxy ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {"providers": [{
  "type": "Crowd",
  "displayName": "Crowd",
  "crowd": {
    "baseURL": "https://crowd.example.com/crowd",
    "clientID": "plainstring",
    "clientSecret": "plainstring",
    "enableBasicAuth": true,
    "groups": [
      "administrators",
      "users"
    ]
  }
}]}, "publishAPI": {"enabled": true}}}`, "")

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.KubeStateSet(``)
			testCreateJobPod()
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Certificate should be generated", func() {
			Expect(f.ValuesGet("userAuthn.internal.crowdProxyCert").String()).To(BeEquivalentTo(testingCert))
		})

		It("Should generate job with a valid image", func() {
			registry := f.ValuesGet("global.modulesImages.registry.base").String()
			digest := f.ValuesGet("global.modulesImages.digests.userAuthn.cfssl").String()
			job := generateJob(registry, digest, "dGVzdAo=")
			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Containers[0].Image).To(ContainSubstring("@"))
			Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal(registry + "@" + digest))
		})
	})

	Context("Cluster with existing cert", func() {
		existingCert := "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJZakNDQVF5Z0F3SUJBZ0lCQVRBTkJna3Foa2lHOXcwQkFRc0ZBREFkTVJzd0dRWURWUVFLRXhKbWNtOXUKZEMxd2NtOTRlUzFqYkdsbGJuUXdJQmNOTURreE1URXdNak13TURBd1doZ1BNakV3T1RFd01UY3lNekF3TURCYQpNQjB4R3pBWkJnTlZCQW9URW1aeWIyNTBMWEJ5YjNoNUxXTnNhV1Z1ZERCY01BMEdDU3FHU0liM0RRRUJBUVVBCkEwc0FNRWdDUVFDaFI4ak9PRncxV29zL2F1WmxsK1QyejZqM2w4SlJSK1lhOVhWZTVDTm9zV3NtQ1RxNHVxWGoKS3QxNnNDdWsvMDVqZFo2bFh2Y1BmaWo4bitzaGFhaERBZ01CQUFHak5UQXpNQTRHQTFVZER3RUIvd1FFQXdJRgpvREFUQmdOVkhTVUVEREFLQmdnckJnRUZCUWNEQVRBTUJnTlZIUk1CQWY4RUFqQUFNQTBHQ1NxR1NJYjNEUUVCCkN3VUFBMEVBblZpM1Y3cUljTEFsYVZZME8xckpqUk1jY0NjdlZFdW5sdzJ4M2d5Wld1SHZwbDM4d3RNVytkVFUKd2NyVWFudFJEY2xIbkJwR1JMazhCTzZsRE9yN0N3PT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo="
		existingKey := "LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlCT1FJQkFBSkJBS0ZIeU00NFhEVmFpejlxNW1XWDVQYlBxUGVYd2xGSDVocjFkVjdrSTJpeGF5WUpPcmk2CnBlTXEzWHF3SzZUL1RtTjFucVZlOXc5K0tQeWY2eUZwcUVNQ0F3RUFBUUpBUVg4OGpuc1cvMWZwQ3ZVbjRnUkEKcVBjR1lKNlIvSjVkVlg5dmpmektZSDVmdHZYNDluZUswaXRleTNSTzV4bEhteWVLNkt2QmlyQnpCd3VPb0V0WAo0UUloQU1hTXRMU1pPY2RYYUVoUG9SMHJCWUlEbmdSTnprUEJiakplM2VMc2Vhb1RBaUVBei9KcFQzNmVBdElTCkN1Ym81OVdoUitjQWVNekFsdXo0MEdNVUlWbnd6eEVDSUZBN2ZhNVpHTHNQL0NqMFhLTy94Y3NERVRDbURFcmUKK0Z2TWNCZUovYVFYQWlCOUlyVjQzd3NiUzJzTUlIU2J2cFQxZmU5c3dscEsrSU9xYzFVVDFObnk0UUlnSVZhdQo1bmdsZ2pqYmQ5b1VxTnNzZS92SGV6SzlIQUZiczhRSXdSL3dJUGs9Ci0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg=="
		BeforeEach(func() {
			f.KubeStateSet(fmt.Sprintf(`
apiVersion: v1
kind: Secret
type: Opaque
data:
  client.crt: %s
  client.key: %s
metadata:
  name: crowd-basic-auth-cert
  namespace: d8-user-authn
`, existingCert, existingKey))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should keep previous certificate and key", func() {
			Expect(f.ValuesGet("userAuthn.internal.crowdProxyCert").String()).To(BeEquivalentTo(existingCert))
			Expect(f.ValuesGet("userAuthn.internal.crowdProxyKey").String()).To(BeEquivalentTo(existingKey))
		})
	})

	Context("Cluster with expired cert", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: Secret
type: Opaque
data:
  client.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJZRENDQVFxZ0F3SUJBZ0lCQVRBTkJna3Foa2lHOXcwQkFRc0ZBREFkTVJzd0dRWURWUVFLRXhKbWNtOXUKZEMxd2NtOTRlUzFqYkdsbGJuUXdIaGNOTURreE1URXdNak13TURBd1doY05NRGt4TVRFd01qTXdNREF4V2pBZApNUnN3R1FZRFZRUUtFeEptY205dWRDMXdjbTk0ZVMxamJHbGxiblF3WERBTkJna3Foa2lHOXcwQkFRRUZBQU5MCkFEQklBa0VBdlZSQkZRbVhCQVNBanhwanQxZjFoRCtNam9jSm16R3pUU1B0b055ZG5iVDBwTTNyazBqSHdlTmgKemdQMUdQRVRoN1pqcWVTbzdHSEZSbU92bk1BbGlRSURBUUFCb3pVd016QU9CZ05WSFE4QkFmOEVCQU1DQmFBdwpFd1lEVlIwbEJBd3dDZ1lJS3dZQkJRVUhBd0V3REFZRFZSMFRBUUgvQkFJd0FEQU5CZ2txaGtpRzl3MEJBUXNGCkFBTkJBRllKWk5SU01jVVlhYnhBdFZTUWNxWVZCTnpuemEzKzhLV1o3RmpUOURsbXFsM0FWR29YcWcyT3pjWVcKU2V6WnFtaEQwTzR3ZUkyb0orOHRQMU5qQ1hrPQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
  client.key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlCT1FJQkFBSkJBTDFVUVJVSmx3UUVnSThhWTdkWDlZUS9qSTZIQ1pzeHMwMGo3YURjbloyMDlLVE42NU5JCng4SGpZYzREOVJqeEU0ZTJZNm5rcU94aHhVWmpyNXpBSllrQ0F3RUFBUUpBQmlPT1BLMWo3U2hzTnJlblZoR1AKRDJ1MEZnY0E0N3hYMFArQ08vNExTa3ErYUh3RE5xcFp5WDVFQlMwWm00emd6dTVHQm9weE4vNllmSUw3YXg4VQpkUUloQVBjM1ErYjRKTTdyTE1xWGVzNFBTYllTcVYvU2RyQzRCRHg2QXBZOHNpVkxBaUVBeEE1djZZbjNPUkYvCkhNTDlic0tFZ1lBK3AzYUR6YjhNcEFNRU9ZZEVuL3NDSUJDaW5XVWJhWTZxOEphcFh0QWk0empuUkpKNEhSaUQKS1hYUVdBQTRFVnpGQWlBOWttTW5Md01MVXlsZWVRWnFrSUJZdzFQcDk5aHc5ejBiRFM5NGViamRuUUlnTDBjbQp6dFhxRFVrUWRIbGxtRlB2Y05IZWlWYXVGaG5qVGUzMjE5TmRmSkU9Ci0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
metadata:
  name: crowd-basic-auth-cert
  namespace: d8-user-authn
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			testCreateJobPod()
			f.RunHook()
		})

		It("New cert should be generated", func() {
			Expect(f.ValuesGet("userAuthn.internal.crowdProxyCert").String()).To(BeEquivalentTo(testingCert))
		})
	})
})

func testCreateJobPod() {
	_, _ = dependency.TestDC.MustGetK8sClient().CoreV1().Pods("d8-system").Create(context.Background(), &corev1.Pod{
		TypeMeta: v1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "testpod",
			Namespace: "d8-system",
			Labels: map[string]string{
				"job-name": "crowd-proxy-cert-generate-job",
			},
		},
	}, v1.CreateOptions{})
}
