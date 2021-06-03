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
				Licence:     "Apache License 2.0",
			},
		},
		{
			name:    "empty description leads to error",
			wantErr: true,
			project: ossProject{
				Name:    "Dex",
				Link:    "https://github.com/dexidp/dex",
				Logo:    "https://dexidp.io/img/logos/dex-horizontal-color.png",
				Licence: "Apache License 2.0",
			},
		},
		{
			name:    "empty link leads to error",
			wantErr: true,
			project: ossProject{
				Name:        "Dex",
				Description: "A Federated OpenID Connect Provider with pluggable connectors",
				Logo:        "https://dexidp.io/img/logos/dex-horizontal-color.png",
				Licence:     "Apache License 2.0",
			},
		},
		{
			name:    "empty logo is optional, does not lead to error",
			wantErr: false,
			project: ossProject{
				Name:        "Dex",
				Description: "A Federated OpenID Connect Provider with pluggable connectors",
				Link:        "https://github.com/dexidp/dex",
				Licence:     "Apache License 2.0",
			},
		},
		{
			name:    "empty licence leads to error",
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
				Licence:     "Apache License 2.0",
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
				Licence:     "Apache License 2.0",
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
  licence: Opachke 2.0
`,
		},
		{
			name:      "two",
			wantCount: 2,
			yaml: `
- name: a
  description: a
  link: https://example.com
  licence: Opachke 2.0
- name: b
  description: b
  link: https://example.com
  licence: Opachke 2.0
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
