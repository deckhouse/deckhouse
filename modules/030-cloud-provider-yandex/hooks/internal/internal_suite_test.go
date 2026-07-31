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

package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestInternal(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Internal Suite")
}

var _ = BeforeSuite(func() {
	// Pre-initialise the global SchemaStore so that ValidateDiscoveryData
	// and ParseConfigFromData work in unit tests without panicking on a
	// missing /deckhouse/candi directory.
	initSchemaStore()
})

// initSchemaStore ensures the global config.SchemaStore is initialised with
// the Yandex OpenAPI schemas so that ValidateDiscoveryData and
// ParseConfigFromData succeed in unit tests.
func initSchemaStore() {
	repoRoot := findRepoRoot()
	if repoRoot == "" {
		return
	}

	globalCandi := filepath.Join(repoRoot, "candi")
	yandexCandi := filepath.Join(repoRoot, "modules", "030-cloud-provider-yandex", "candi")

	for _, d := range []string{globalCandi, yandexCandi} {
		if _, statErr := os.Stat(d); statErr != nil {
			return
		}
	}

	_ = config.NewSchemaStore(
		&options.GlobalOptions{CandiDir: globalCandi},
		yandexCandi,
	)
}
