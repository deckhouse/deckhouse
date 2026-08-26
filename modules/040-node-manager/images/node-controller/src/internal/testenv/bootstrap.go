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

package testenv

import (
	"os"
	"path/filepath"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/node-controller/internal/bootstrap"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// bootstrapTemplateFiles are the candi templates the chart puts in the
// ConfigMap, keyed as it keys them (templates/bashible/bootstrap-templates-cm.yaml).
// Provider network scripts are left out: they live next to their cloud-provider
// module and a checkout has no candi/cloud-providers directory.
var bootstrapTemplateFiles = map[string]string{
	"candi/bashible/lib.sh.tpl":                                  "lib.sh.tpl",
	"candi/bashible/bootstrap/01-bootstrap-prerequisites.sh.tpl": "01-bootstrap-prerequisites.sh.tpl",
	"candi/bashible/bb_node_ip.sh.tpl":                           "bb_node_ip.sh.tpl",
}

// BootstrapTemplatesConfigMap builds the ConfigMap the chart fills with the candi
// bootstrap templates, read from the repository so a suite renders what a release
// ships rather than a stub. minget is stubbed: only its base64 is inlined.
func BootstrapTemplatesConfigMap() *corev1.ConfigMap {
	ginkgo.GinkgoHelper()

	root := repoRoot()
	data := make(map[string]string, len(bootstrapTemplateFiles))
	for path, key := range bootstrapTemplateFiles {
		raw, err := os.ReadFile(filepath.Join(root, path))
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "read %s", path)
		data[key] = string(raw)
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.MachineNamespace, Name: bootstrap.TemplatesConfigMapName},
		Data:       data,
		BinaryData: map[string][]byte{"minget": []byte("minget")},
	}
}

// repoRoot resolves the repository root from this file's build-time location, so
// it holds whatever directory the calling test package runs from.
func repoRoot() string {
	return filepath.Join(filepath.Dir(callerFile()), "..", "..", "..", "..", "..", "..", "..")
}
