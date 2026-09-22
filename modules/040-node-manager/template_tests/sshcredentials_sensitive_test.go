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

package template_tests

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"
)

// The markers are hand-written into the CRD: controller-gen does not emit
// x-kubernetes-sensitive-data and would drop the x-doc-* keys the file already
// carries. Regenerating the CRD would therefore silently strip them, the
// apiserver would go back to serving the SSH key and the sudo password to
// everyone who can read sshcredentials, and no other test would notice — the
// CAPS ClusterRole rule they pair with stays valid either way.
const sshCredentialsCRDPath = "../crds/sshcredentials.yaml"

// Fields whose values must never reach a caller without the
// sshcredentials/sensitive subresource, per served version. Both versions are
// listed on purpose: masking applies the union of the schemas of every version
// that carries a marker, so a served version left unmarked leaks whichever key
// name only it uses — sudoPassword for v1alpha1, sudoPasswordEncoded for v1alpha2.
var sshCredentialsSensitiveFields = map[string][]string{
	"v1alpha1": {"privateSSHKey", "sudoPassword"},
	"v1alpha2": {"privateSSHKey", "sudoPasswordEncoded"},
}

// Marking spec as a whole would hide these too, leaving a viewer unable to tell
// one SSHCredentials from another.
var sshCredentialsVisibleFields = []string{"user", "sshPort", "sshExtraArgs"}

var _ = Describe("Module :: node-manager :: SSHCredentials CRD :: sensitive fields ::", func() {
	specProperties := func() map[string]map[string]interface{} {
		raw, err := os.ReadFile(sshCredentialsCRDPath)
		Expect(err).ShouldNot(HaveOccurred())

		// Deliberately untyped: x-kubernetes-sensitive-data is a Deckhouse
		// extension to apiextensions-apiserver, so decoding into the upstream
		// CustomResourceDefinition struct would drop the very key under test.
		var crd map[string]interface{}
		Expect(yaml.Unmarshal(raw, &crd)).To(Succeed())

		spec, ok := crd["spec"].(map[string]interface{})
		Expect(ok).To(BeTrue(), "spec is missing in %s", sshCredentialsCRDPath)
		versions, ok := spec["versions"].([]interface{})
		Expect(ok).To(BeTrue(), "spec.versions is missing in %s", sshCredentialsCRDPath)

		byVersion := map[string]map[string]interface{}{}
		for _, v := range versions {
			version, ok := v.(map[string]interface{})
			Expect(ok).To(BeTrue())

			name, ok := version["name"].(string)
			Expect(ok).To(BeTrue(), "version without a name in %s", sshCredentialsCRDPath)

			// An unserved version is unreachable, so it cannot leak.
			if served, ok := version["served"].(bool); ok && !served {
				continue
			}

			properties, ok := version["schema"].(map[string]interface{})["openAPIV3Schema"].(map[string]interface{})["properties"].(map[string]interface{})["spec"].(map[string]interface{})["properties"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "cannot reach spec.properties of version %s", name)

			byVersion[name] = properties
		}
		return byVersion
	}

	It("marks the secrets of every served version with x-kubernetes-sensitive-data", func() {
		properties := specProperties()

		// Fails when a version is added or stops being served, which is the
		// point: the new version needs its own markers, or masking skips it.
		Expect(properties).To(HaveLen(len(sshCredentialsSensitiveFields)))

		for version, fields := range sshCredentialsSensitiveFields {
			Expect(properties).To(HaveKey(version))

			for _, field := range fields {
				property, ok := properties[version][field].(map[string]interface{})
				Expect(ok).To(BeTrue(), "%s: spec.%s is missing", version, field)
				Expect(property["x-kubernetes-sensitive-data"]).To(BeTrue(),
					"%s: spec.%s must carry x-kubernetes-sensitive-data: true, otherwise the apiserver serves it to anyone who can read sshcredentials", version, field)
			}
		}
	})

	It("keeps the non-secret fields readable", func() {
		properties := specProperties()

		for version := range sshCredentialsSensitiveFields {
			for _, field := range sshCredentialsVisibleFields {
				property, ok := properties[version][field].(map[string]interface{})
				Expect(ok).To(BeTrue(), "%s: spec.%s is missing", version, field)
				Expect(property).ToNot(HaveKey("x-kubernetes-sensitive-data"),
					"%s: spec.%s must stay visible — mark the secret fields one by one, never spec as a whole", version, field)
			}
		}
	})
})
