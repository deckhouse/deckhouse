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
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"
)

// pulledDexCRDs returns the manifests crds/pull_dex_crds.sh copies from the patched Dex, which
// the Dex image build checks against the Dex it builds.
func pulledDexCRDs() []string {
	script, err := os.ReadFile(filepath.Join("..", "crds", "pull_dex_crds.sh"))
	Expect(err).ToNot(HaveOccurred())

	var crds []string
	inList := false
	for _, line := range strings.Split(string(script), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "crds=(":
			inList = true
		case inList && line == ")":
			return crds
		case inList:
			crds = append(crds, line)
		}
	}
	Fail("no crds list in pull_dex_crds.sh")
	return nil
}

var _ = Describe("User Authn hooks :: ensure external crds ::", func() {
	// The hook installs every manifest in crds/external. Those are to be the manifests the image
	// build checks against the patched Dex: a manifest outside the list would be installed
	// unchecked, one missing from the directory never installed.
	It("Should install exactly the Dex CRDs the image build checks", func() {
		files, err := filepath.Glob(filepath.Join("..", "crds", "external", "*.yaml"))
		Expect(err).ToNot(HaveOccurred())
		installed := make([]string, 0, len(files))
		for _, file := range files {
			installed = append(installed, filepath.Base(file))
		}

		Expect(installed).To(ConsistOf(pulledDexCRDs()))
		// Dex writes revocations at logout and at /token/revoke (patch 290).
		Expect(installed).To(ContainElements("useridentities.yaml", "authsessions.yaml", "revocations.yaml"))
	})

	It("Should install the CRDs of the Dex storage group only", func() {
		for _, name := range pulledDexCRDs() {
			raw, err := os.ReadFile(filepath.Join("..", "crds", "external", name))
			Expect(err).ToNot(HaveOccurred())

			var crd struct {
				Kind     string `json:"kind"`
				Metadata struct {
					Name string `json:"name"`
				} `json:"metadata"`
				Spec struct {
					Group string `json:"group"`
					Scope string `json:"scope"`
					Names struct {
						Plural string `json:"plural"`
					} `json:"names"`
				} `json:"spec"`
			}
			Expect(yaml.Unmarshal(raw, &crd)).To(Succeed(), name)
			Expect(crd.Kind).To(Equal("CustomResourceDefinition"), name)
			Expect(crd.Spec.Group).To(Equal("dex.coreos.com"), name)
			Expect(crd.Spec.Scope).To(Equal("Namespaced"), name)
			Expect(crd.Spec.Names.Plural + ".yaml").To(Equal(name))
			Expect(crd.Metadata.Name).To(Equal(crd.Spec.Names.Plural+".dex.coreos.com"), name)
		}
	})
})
