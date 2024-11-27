/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	"io/fs"
	"testing"
)

func Test_TemplatesExists(t *testing.T) {
	count := 0

	fs.WalkDir(templatesFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Errorf("walk error: %v", err)
		}

		if d.IsDir() {
			return nil
		}

		t.Logf("- %v", path)

		count++

		return nil
	})

	t.Logf("Templates found: %v", count)

	if count == 0 {
		t.Errorf("no templates found")
	}
}

func Test_TemplatesLoading(t *testing.T) {
	matrix := map[string]templateName{
		"auth config":         authConfigTemplateName,
		"distribution config": distributionConfigTemplateName,
		"registry static pod": registryStaticPodTemplateName,
	}

	for k, tpl := range matrix {
		buf, err := getTemplateContent(tpl)

		if err != nil {
			t.Errorf("Cannot load %v template: %v", k, err)
		}

		size := len(buf)

		if size == 0 {
			t.Errorf("Template %v content is empty!", k)
		}

		t.Logf("- %v template: { path: %v, size: %v }", k, tpl, size)
	}
}
