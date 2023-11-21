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
			"/app/hugo/content/moduleName/stable/install.md",
			true,
		},
		{
			"./docs",
			"/app/hugo/content/moduleName/stable",
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
	}
	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			u := newLoadHandler("/app/hugo/")

			got, ok := u.getLocalPath("moduleName", "stable", tt.fileName)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("getLocalPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
