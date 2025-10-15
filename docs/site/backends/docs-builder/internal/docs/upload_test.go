// Copyright 2023 Flant JSC
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

package docs

import (
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

func TestLoadHandlerGetLocalPath(t *testing.T) {
	tests := []struct {
		fileName string
		want     string
		wantOK   bool
	}{
		{
			"./docs/install.md",
			"/app/hugo/content/modules/moduleName/stable/install.md",
			true,
		},
		{
			"./docs",
			"/app/hugo/content/modules/moduleName/stable",
			true,
		},
		{
			"docs/install.md",
			"/app/hugo/content/modules/moduleName/stable/install.md",
			true,
		},
		{
			"docs/README_RU.md",
			"/app/hugo/content/modules/moduleName/stable/README.ru.md",
			true,
		},
		{
			"docs",
			"/app/hugo/content/modules/moduleName/stable",
			true,
		},
		{
			"docs/install.md",
			"/app/hugo/content/modules/moduleName/stable/install.md",
			true,
		},
		{
			"docs/README_RU.md",
			"/app/hugo/content/modules/moduleName/stable/README.ru.md",
			true,
		},
		{
			"docs",
			"/app/hugo/content/modules/moduleName/stable",
			true,
		},
		{
			"not-docs/file.ext",
			"",
			false,
		},
		{
			"crds/object.yaml",
			"/app/hugo/data/modules/moduleName/stable/crds/object.yaml",
			true,
		},
		{
			"openapi/doc-ru-config-values.yaml",
			"/app/hugo/data/modules/moduleName/stable/openapi/doc-ru-config-values.yaml",
			true,
		},
		{
			"openapi/openapi-case-tests.yaml",
			"",
			false,
		},
		{
			"./openapi/config-values.yaml",
			"/app/hugo/data/modules/moduleName/stable/openapi/config-values.yaml",
			true,
		},
		{
			"openapi",
			"/app/hugo/data/modules/moduleName/stable/openapi",
			true,
		},
		{
			"openapi",
			"/app/hugo/data/modules/moduleName/stable/openapi",
			true,
		},
		// Test cases for internal directories exclusion
		{
			"docs/internal/README.md",
			"",
			false,
		},
		{
			"docs/internals/development.md",
			"",
			false,
		},
		{
			"docs/development/HOWTO.md",
			"",
			false,
		},
		{
			"docs/dev/debug.md",
			"",
			false,
		},
		{
			"docs/internal/subfolder/file.md",
			"",
			false,
		},
		// Test that regular docs files still work
		{
			"docs/public/README.md",
			"/app/hugo/content/modules/moduleName/stable/public/README.md",
			true,
		},
		{
			"docs/configuration.md",
			"/app/hugo/content/modules/moduleName/stable/configuration.md",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			var svc = NewService("/app/hugo/", "", false, log.NewNop(), metricsstorage.NewMetricStorage(""))

			got, ok := svc.getLocalPath("moduleName", "stable", tt.fileName)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("getLocalPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
