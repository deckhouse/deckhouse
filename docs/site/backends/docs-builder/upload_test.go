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
