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

package v2

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	registrypki "github.com/deckhouse/deckhouse/go_lib/registry/pki"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/v2/pki"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// TestThePersistedPKIIsFoundWhereverItStillIs is about not reissuing a certificate authority.
//
// The state moved out of the secret the storage pod mounts, because that mount handed every
// container in the pod the authority's private key and the token signing key. Moving it creates one
// moment where it matters which secret is read: a cluster that was running the previous render has
// the state only in the old place, and the new secret cannot appear until the hook has put the state
// into the values it is rendered from. Read only the new name and that cluster generates a fresh
// authority — every node agent then trusts a certificate the storage no longer presents, and every
// pull in the cluster fails until the layouts have been rewritten and applied.
func TestThePersistedPKIIsFoundWhereverItStillIs(t *testing.T) {
	cases := []struct {
		name  string
		found []pkiStateSnapshot
		want  string
	}{{
		name: "nothing persisted yet, so the hook generates",
		want: "",
	}, {
		name:  "the state secret alone, which is the ordinary case",
		found: []pkiStateSnapshot{{Secret: PKIStateSecretName, State: []byte("new")}},
		want:  "new",
	}, {
		// A cluster upgraded from the render that kept the state in the mounted secret.
		name:  "only the old location still has it",
		found: []pkiStateSnapshot{{Secret: PKISecretName, State: []byte("old")}},
		want:  "old",
	}, {
		// Both exist for exactly one reconciliation, until the mounted secret is re-rendered
		// without the state. The one the hook now writes wins.
		name: "both, during the change",
		found: []pkiStateSnapshot{
			{Secret: PKISecretName, State: []byte("old")},
			{Secret: PKIStateSecretName, State: []byte("new")},
		},
		want: "new",
	}, {
		name:  "a secret that carries no state at all",
		found: []pkiStateSnapshot{{Secret: PKIStateSecretName}},
		want:  "",
	}}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, string(preferredPKIState(testCase.found)))
		})
	}
}

// TestStorePathFollowsTheMasterGroup: where the store keeps its blobs is a property of the
// node it runs on, not of the configuration.
//
// Both directions are a silent failure, and they are not symmetric. On an Engine cluster
// /opt is read-only, so the wrong answer is a store that cannot start — loudly, in the
// pod. On a bashible cluster the wrong answer is a store pointed at an empty directory
// beside the one holding every image the cluster already has: nothing fails, the cache
// simply refetches the lot, and the blobs it had keep the disk.
func TestStorePathFollowsTheMasterGroup(t *testing.T) {
	cases := []struct {
		name       string
		systemType string
		want       string
	}{{
		// A group predating the field carries no systemType, and that absence is bashible.
		name:       "a group that names no system type",
		systemType: "",
		want:       registry_const.StorePath,
	}, {
		name:       "a bashible master group",
		systemType: "Mutable",
		want:       registry_const.StorePath,
	}, {
		name:       "an Engine master group",
		systemType: systemTypeImmutable,
		want:       registry_const.StorePathImmutable,
	}}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, storePathFor(testCase.systemType))
		})
	}
}

// The two paths must differ, and the Engine one must not be under /opt: that is the whole
// reason the second constant exists.
func TestTheImmutableStorePathIsNotUnderOpt(t *testing.T) {
	assert.NotEqual(t, registry_const.StorePath, registry_const.StorePathImmutable)
	assert.False(t, strings.HasPrefix(registry_const.StorePathImmutable, "/opt/"),
		"a read-only root filesystem has nowhere to put blobs under /opt")
}

// A cluster generated before the ingress client key was written in PKCS#8 has, in its
// stored state, the shared encoder's PKCS#1 under the PKCS#8 label. ingress-nginx loads
// that key with OpenSSL, which refuses it, and nginx then refuses its whole
// configuration. The next run of the hook has to heal it from what is stored, without
// regenerating the authority every node trusts.
var _ = Describe("Modules :: registry :: hooks :: v2 :: storage PKI ::", func() {
	f := HookExecutionConfigInit(`{"registry":{"internal":{"v2":{"enabled":false}}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	var stored pki.State

	BeforeEach(func() {
		generated, err := pki.Generate(pki.Hosts{Service: ServiceName})
		Expect(err).ShouldNot(HaveOccurred())

		key, err := registrypki.DecodePrivateKey([]byte(generated.IngressClient.Key))
		Expect(err).ShouldNot(HaveOccurred())
		legacy, err := registrypki.EncodePrivateKey(key)
		Expect(err).ShouldNot(HaveOccurred())
		generated.IngressClient.Key = string(legacy)
		Expect(strictPKCS8(generated.IngressClient.Key)).ShouldNot(Succeed(),
			"the stored key this test starts from must be the one OpenSSL refuses")

		stored = generated
	})

	expectHealed := func() {
		Expect(f).To(ExecuteSuccessfully())

		values := f.ValuesGet("registry.internal.v2.pki")
		Expect(values.Get("ca.key").String()).To(Equal(stored.CA.Key), "the authority is not regenerated")
		Expect(values.Get("ingressClient.cert").String()).To(Equal(stored.IngressClient.Cert))
		Expect(strictPKCS8(values.Get("ingressClient.key").String())).To(Succeed())

		before, err := registrypki.DecodePrivateKey([]byte(stored.IngressClient.Key))
		Expect(err).ShouldNot(HaveOccurred())
		after, err := registrypki.DecodePrivateKey([]byte(values.Get("ingressClient.key").String()))
		Expect(err).ShouldNot(HaveOccurred())
		Expect(after.Public()).To(Equal(before.Public()), "re-encoded, not reissued")
	}

	Context("restored from the state secret, as after an upgrade", func() {
		BeforeEach(func() {
			raw, err := yaml.Marshal(stored)
			Expect(err).ShouldNot(HaveOccurred())

			f.BindingContexts.Set(f.KubeStateSet(fmt.Sprintf(`
---
apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: d8-system
data:
  state.yaml: %s
`, PKIStateSecretName, base64.StdEncoding.EncodeToString(raw))))
			f.RunHook()
		})

		It("re-encodes the ingress client key and keeps everything else", expectHealed)
	})

	Context("already in the values", func() {
		BeforeEach(func() {
			raw, err := yaml.Marshal(stored)
			Expect(err).ShouldNot(HaveOccurred())
			f.ValuesSetFromYaml("registry.internal.v2.pki", raw)
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("re-encodes the ingress client key and keeps everything else", expectHealed)
	})
})

// A stored ingress client pair whose key is not the certificate's cannot be re-encoded.
// The hook used to fail on it on every run, and nothing would ever change the stored
// state; it reissues the pair from the same authority instead.
var _ = Describe("Modules :: registry :: hooks :: v2 :: storage PKI :: mismatched ingress client ::", func() {
	f := HookExecutionConfigInit(`{"registry":{"internal":{"v2":{"enabled":false}}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	var stored pki.State

	BeforeEach(func() {
		generated, err := pki.Generate(pki.Hosts{Service: ServiceName})
		Expect(err).ShouldNot(HaveOccurred())
		generated.IngressClient.Key = generated.Distribution.Key
		stored = generated

		raw, err := yaml.Marshal(stored)
		Expect(err).ShouldNot(HaveOccurred())
		f.ValuesSetFromYaml("registry.internal.v2.pki", raw)
		f.BindingContexts.Set(f.KubeStateSet(""))
		f.RunHook()
	})

	It("reissues the ingress client pair under the same authority", func() {
		Expect(f).To(ExecuteSuccessfully())

		values := f.ValuesGet("registry.internal.v2.pki")
		Expect(values.Get("ca.key").String()).To(Equal(stored.CA.Key), "the authority is not regenerated")
		Expect(values.Get("distribution.cert").String()).To(Equal(stored.Distribution.Cert))

		cert := values.Get("ingressClient.cert").String()
		key := values.Get("ingressClient.key").String()
		Expect(cert).NotTo(Equal(stored.IngressClient.Cert))
		Expect(strictPKCS8(key)).To(Succeed())
		_, err := registrypki.DecodeCertKey([]byte(cert), []byte(key))
		Expect(err).ShouldNot(HaveOccurred(), "the key is the certificate's")

		issued, err := registrypki.DecodeCertificate([]byte(cert))
		Expect(err).ShouldNot(HaveOccurred())
		authority, err := registrypki.DecodeCertificate([]byte(stored.CA.Cert))
		Expect(err).ShouldNot(HaveOccurred())
		Expect(issued.CheckSignatureFrom(authority)).To(Succeed())
	})
})

// strictPKCS8 is what OpenSSL requires of a key labelled "PRIVATE KEY", checked with the
// strict parser rather than the lenient ones the module's Go readers use.
func strictPKCS8(key string) error {
	block, rest := pem.Decode([]byte(key))
	if block == nil || len(rest) != 0 || block.Type != "PRIVATE KEY" {
		return fmt.Errorf("not a single PRIVATE KEY block")
	}
	_, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	return err
}
