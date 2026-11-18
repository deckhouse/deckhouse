// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package credentials

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestModuleSourceSignaturePolicy(t *testing.T) {
	const deckhouseRepository = "registry.test/deckhouse"

	for _, repository := range []string{"registry.test/modules", deckhouseRepository, "registry.test"} {
		t.Run(repository, func(t *testing.T) {
			w := &Watcher{
				logger: log.NewNop(),
				registryClientConfigs: map[string]*registry.ClientConfig{
					registry.DefaultRepository: {Repository: deckhouseRepository},
				},
			}
			event := watch.Event{Type: watch.Added, Object: &unstructured.Unstructured{Object: map[string]any{
				"spec": map[string]any{"registry": map[string]any{"repo": repository}},
			}}}
			require.NoError(t, w.processModuleSourceEvent(event))

			cfg, err := w.Get(repository)
			require.NoError(t, err)
			require.Equal(t, repository == "registry.test/modules", cfg.SkipSignatureCheck)
			require.True(t, w.registryClientConfigs[repository].SkipSignatureCheck)

			defaultConfig, err := w.Get(registry.DefaultRepository)
			require.NoError(t, err)
			require.False(t, defaultConfig.SkipSignatureCheck)
		})
	}
}
