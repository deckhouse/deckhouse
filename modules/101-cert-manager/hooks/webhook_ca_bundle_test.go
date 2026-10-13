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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func generateTestCA(cn string) string {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).NotTo(HaveOccurred())

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	Expect(err).NotTo(HaveOccurred())

	return strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})))
}

func webhookConfigurationManifest(trusted ...string) string {
	bundle := base64.StdEncoding.EncodeToString([]byte(strings.Join(trusted, "\n")))
	return fmt.Sprintf(`
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: cert-manager-webhook
webhooks:
- name: webhook.cert-manager.io
  clientConfig:
    caBundle: %s
`, bundle)
}

func webhookDeploymentManifest(ca string, replicas, updated, available int) string {
	return webhookDeploymentManifestWithGeneration(caChecksum(ca), 2, 2, replicas, updated, available)
}

func webhookDeploymentManifestWithGeneration(checksum string, generation, observed, replicas, updated, available int) string {
	return fmt.Sprintf(`
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: webhook
  namespace: d8-cert-manager
  generation: %d
spec:
  replicas: 3
  template:
    metadata:
      annotations:
        checksum/ca: %q
status:
  observedGeneration: %d
  replicas: %d
  updatedReplicas: %d
  availableReplicas: %d
`, generation, checksum, observed, replicas, updated, available)
}

func webhookPodManifest(name, ca string, terminating bool) string {
	return webhookPodManifestInPhase(name, caChecksum(ca), terminating, "Running")
}

func webhookPodManifestInPhase(name, checksum string, terminating bool, phase string) string {
	deletion := ""
	if terminating {
		deletion = "\n  deletionTimestamp: \"2026-01-01T00:00:00Z\""
	}
	return fmt.Sprintf(`
---
apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: d8-cert-manager%s
  labels:
    app: webhook
  annotations:
    checksum/ca: %q
status:
  phase: %s
`, name, deletion, checksum, phase)
}

var _ = Describe("Cert Manager hooks :: webhook CA bundle ::", func() {
	f := HookExecutionConfigInit(`{"certManager":{"internal":{"webhookCert":{}}}}`, "")

	oldCA := generateTestCA("old-webhook-ca")
	newCA := generateTestCA("new-webhook-ca")

	bundleCAs := func() []string {
		var out []string
		for _, b := range pemCertificates(f.ValuesGet(caBundleValuesPath).String()) {
			out = append(out, strings.TrimSpace(string(pem.EncodeToMemory(b))))
		}
		return out
	}

	run := func(state string) {
		f.ValuesSet(webhookCAValuesPath, newCA)
		f.BindingContexts.Set(f.KubeStateSet(state))
		f.RunHook()
		Expect(f).To(ExecuteSuccessfully())
	}

	Context("First installation, nothing in the cluster yet", func() {
		BeforeEach(func() { run("") })

		It("trusts only the current CA", func() {
			Expect(bundleCAs()).To(Equal([]string{newCA}))
			Expect(f.ValuesGet(caRotationActiveValuesPath).Bool()).To(BeFalse())
		})
	})

	Context("The webhook configuration already trusts the current CA", func() {
		BeforeEach(func() { run(webhookConfigurationManifest(newCA) + webhookDeploymentManifest(newCA, 3, 3, 3)) })

		It("trusts only the current CA", func() {
			Expect(bundleCAs()).To(Equal([]string{newCA}))
			Expect(f.ValuesGet(caRotationActiveValuesPath).Bool()).To(BeFalse())
		})
	})

	Context("The CA has just been regenerated and the pods still serve the old certificate", func() {
		BeforeEach(func() { run(webhookConfigurationManifest(oldCA) + webhookDeploymentManifest(oldCA, 3, 3, 3)) })

		It("trusts both CAs and reports the rotation", func() {
			Expect(bundleCAs()).To(Equal([]string{newCA, oldCA}))
			Expect(f.ValuesGet(caRotationActiveValuesPath).Bool()).To(BeTrue())
		})
	})

	Context("The webhook rolls out the new CA", func() {
		BeforeEach(func() { run(webhookConfigurationManifest(newCA, oldCA) + webhookDeploymentManifest(newCA, 4, 2, 3)) })

		It("keeps the old CA until the rollout ends", func() {
			Expect(bundleCAs()).To(Equal([]string{newCA, oldCA}))
			Expect(f.ValuesGet(caRotationActiveValuesPath).Bool()).To(BeTrue())
		})
	})

	Context("The rollout is stuck with a pod of the old template left, for example on a NotReady master", func() {
		BeforeEach(func() { run(webhookConfigurationManifest(newCA, oldCA) + webhookDeploymentManifest(newCA, 3, 2, 2)) })

		It("keeps trusting both CAs", func() {
			Expect(bundleCAs()).To(Equal([]string{newCA, oldCA}))
		})
	})

	Context("Every pod runs the new template, but one of them is not available yet", func() {
		BeforeEach(func() { run(webhookConfigurationManifest(newCA, oldCA) + webhookDeploymentManifest(newCA, 3, 3, 2)) })

		It("drops the old CA, because no pod can serve its certificate any more", func() {
			Expect(bundleCAs()).To(Equal([]string{newCA}))
			Expect(f.ValuesGet(caRotationActiveValuesPath).Bool()).To(BeFalse())
		})
	})

	Context("The new template is applied, but the Deployment status still describes the old one", func() {
		BeforeEach(func() {
			run(webhookConfigurationManifest(newCA, oldCA) + webhookDeploymentManifestWithGeneration(caChecksum(newCA), 3, 2, 3, 3, 3))
		})

		It("keeps the old CA until the controller observes the new generation", func() {
			Expect(bundleCAs()).To(Equal([]string{newCA, oldCA}))
		})
	})

	Context("The Deployment of a previous release has no CA annotation", func() {
		BeforeEach(func() {
			run(webhookConfigurationManifest(oldCA) + webhookDeploymentManifestWithGeneration("", 2, 2, 3, 3, 3))
		})

		It("treats the rollout as not done and trusts both CAs", func() {
			Expect(bundleCAs()).To(Equal([]string{newCA, oldCA}))
			Expect(f.ValuesGet(caRotationActiveValuesPath).Bool()).To(BeTrue())
		})
	})

	Context("The CA rotates again before the previous rotation has finished", func() {
		olderCA := generateTestCA("older-webhook-ca")
		BeforeEach(func() {
			run(webhookConfigurationManifest(oldCA, olderCA) + webhookDeploymentManifest(oldCA, 4, 2, 3))
		})

		It("keeps every CA a pod may still serve", func() {
			Expect(bundleCAs()).To(Equal([]string{newCA, oldCA, olderCA}))
		})
	})

	Context("The caBundle of the webhook configuration is not valid base64", func() {
		BeforeEach(func() {
			f.ValuesSet(webhookCAValuesPath, newCA)
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: cert-manager-webhook
webhooks:
- name: webhook.cert-manager.io
  clientConfig:
    caBundle: not-base64!
`))
			f.RunHook()
		})

		It("falls back to the current CA", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(bundleCAs()).To(Equal([]string{newCA}))
		})
	})

	Context("The CA is not in values yet", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(webhookConfigurationManifest(oldCA)))
			f.RunHook()
		})

		It("leaves the bundle to the template fallback", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(caBundleValuesPath).Exists()).To(BeFalse())
			Expect(f.ValuesGet(caRotationActiveValuesPath).Bool()).To(BeFalse())
		})
	})

	Context("The status counts only new pods, but the last old pod is still terminating", func() {
		BeforeEach(func() {
			run(webhookConfigurationManifest(newCA, oldCA) + webhookDeploymentManifest(newCA, 3, 3, 3) +
				webhookPodManifest("webhook-new-1", newCA, false) +
				webhookPodManifest("webhook-new-2", newCA, false) +
				webhookPodManifest("webhook-new-3", newCA, false) +
				webhookPodManifest("webhook-old-1", oldCA, true))
		})

		It("keeps trusting both CAs until the terminating pod is gone", func() {
			Expect(bundleCAs()).To(Equal([]string{newCA, oldCA}))
			Expect(f.ValuesGet(caRotationActiveValuesPath).Bool()).To(BeTrue())
		})
	})

	Context("The rollout has finished and every pod left runs the new template", func() {
		BeforeEach(func() {
			run(webhookConfigurationManifest(newCA, oldCA) + webhookDeploymentManifest(newCA, 3, 3, 3) +
				webhookPodManifest("webhook-new-1", newCA, false) +
				webhookPodManifest("webhook-new-2", newCA, false) +
				webhookPodManifest("webhook-new-3", newCA, false))
		})

		It("drops the old CA", func() {
			Expect(bundleCAs()).To(Equal([]string{newCA}))
			Expect(f.ValuesGet(caRotationActiveValuesPath).Bool()).To(BeFalse())
		})
	})

	Context("A completed pod of an older template is left after a node shutdown", func() {
		BeforeEach(func() {
			run(webhookConfigurationManifest(newCA, oldCA) + webhookDeploymentManifest(newCA, 3, 3, 3) +
				webhookPodManifest("webhook-new-1", newCA, false) +
				webhookPodManifest("webhook-new-2", newCA, false) +
				webhookPodManifest("webhook-new-3", newCA, false) +
				webhookPodManifestInPhase("webhook-old-1", caChecksum(oldCA), false, "Succeeded") +
				webhookPodManifestInPhase("webhook-older-1", "", false, "Failed"))
		})

		It("drops the old CA, because a completed pod serves nothing", func() {
			Expect(bundleCAs()).To(Equal([]string{newCA}))
			Expect(f.ValuesGet(caRotationActiveValuesPath).Bool()).To(BeFalse())
		})
	})

	Context("The rollout of the new CA has finished", func() {
		BeforeEach(func() { run(webhookConfigurationManifest(newCA, oldCA) + webhookDeploymentManifest(newCA, 3, 3, 3)) })

		It("drops the old CA", func() {
			Expect(bundleCAs()).To(Equal([]string{newCA}))
			Expect(f.ValuesGet(caRotationActiveValuesPath).Bool()).To(BeFalse())
		})
	})
})
