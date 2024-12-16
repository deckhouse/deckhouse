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

package modules

import (
	"testing"
)

func Test_assertProject(t *testing.T) {
	tests := []struct {
		name    string
		project ossProject
		wantErr bool
	}{
		{
			name:    "all empty leads to errors",
			wantErr: true,
			project: ossProject{},
		},
		{
			name:    "all filled leads to no errors",
			wantErr: false,
			project: ossProject{
				Name:        "Dex",
				Description: "A Federated OpenID Connect Provider with pluggable connectors",
				Link:        "https://github.com/dexidp/dex",
				Logo:        "https://dexidp.io/img/logos/dex-horizontal-color.png",
				License:     "Apache License 2.0",
			},
		},
		{
			name:    "empty description leads to error",
			wantErr: true,
			project: ossProject{
				Name:    "Dex",
				Link:    "https://github.com/dexidp/dex",
				Logo:    "https://dexidp.io/img/logos/dex-horizontal-color.png",
				License: "Apache License 2.0",
			},
		},
		{
			name:    "empty link leads to error",
			wantErr: true,
			project: ossProject{
				Name:        "Dex",
				Description: "A Federated OpenID Connect Provider with pluggable connectors",
				Logo:        "https://dexidp.io/img/logos/dex-horizontal-color.png",
				License:     "Apache License 2.0",
			},
		},
		{
			name:    "empty logo is optional, does not lead to error",
			wantErr: false,
			project: ossProject{
				Name:        "Dex",
				Description: "A Federated OpenID Connect Provider with pluggable connectors",
				Link:        "https://github.com/dexidp/dex",
				License:     "Apache License 2.0",
			},
		},
		{
			name:    "empty license leads to error",
			wantErr: true,
			project: ossProject{
				Name:        "Dex",
				Description: "A Federated OpenID Connect Provider with pluggable connectors",
				Link:        "https://github.com/dexidp/dex",
				Logo:        "https://dexidp.io/img/logos/dex-horizontal-color.png",
			},
		},
		{
			name:    "malformed link leads to error",
			wantErr: true,
			project: ossProject{
				Name:        "Dex",
				Description: "A Federated OpenID Connect Provider with pluggable connectors",
				Link:        "zazaz",
				Logo:        "https://dexidp.io/img/logos/dex-horizontal-color.png",
				License:     "Apache License 2.0",
			},
		},
		{
			name:    "malformed logo link leads to error",
			wantErr: true,
			project: ossProject{
				Name:        "Dex",
				Description: "A Federated OpenID Connect Provider with pluggable connectors",
				Link:        "https://github.com/dexidp/dex",
				Logo:        "xoxoxo",
				License:     "Apache License 2.0",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := assertOssProject(0, test.project)
			if test.wantErr {
				if err == nil {
					t.Errorf("expected error, not nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func Test_projectList(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantCount int
	}{
		{
			name:      "empty",
			yaml:      "",
			wantCount: 0,
		},
		{
			name:      "one",
			wantCount: 1,
			yaml: `
- name: a
  description: a
  link: https://example.com
  license: Opachke 2.0
`,
		},
		{
			name:      "two",
			wantCount: 2,
			yaml: `
- name: a
  description: a
  link: https://example.com
  license: Opachke 2.0
- name: b
  description: b
  link: https://example.com
  license: Opachke 2.0
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projects, err := parseProjectList([]byte(test.yaml))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if len(projects) != test.wantCount {
				t.Errorf("unexpected project count: got=%d, want=%d", len(projects), test.wantCount)
			}
		})
	}
}
