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

package main

import "testing"

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
			"/app/hugo/content/moduleName/stable/install.md",
			true,
		},
		{
			"docs/README_RU.md",
			"/app/hugo/content/moduleName/stable/README.ru.md",
			true,
		},
		{
			"docs",
			"/app/hugo/content/moduleName/stable",
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
	}
	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			u := newLoadHandler("/app/hugo/", nil)

			got, ok := u.getLocalPath("moduleName", "stable", tt.fileName)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("getLocalPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
