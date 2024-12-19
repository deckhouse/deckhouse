/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	"bytes"
	"embed"
	"fmt"
	"strconv"
	"text/template"
)

//go:embed templates
var templatesFS embed.FS

type templateName string

const (
	authConfigTemplateName         templateName = "templates/auth/config.yaml.tpl"
	distributionConfigTemplateName templateName = "templates/distribution/config.yaml.tpl"
	registryStaticPodTemplateName  templateName = "templates/static_pods/system-registry.yaml.tpl"
	mirrorerConfigTemplateName     templateName = "templates/mirrorer/config.yaml.tpl"
)

// RenderTemplate renders the provided template content with the given data
func renderTemplate(name templateName, data interface{}) ([]byte, error) {
	content, err := templatesFS.ReadFile(string(name))
	if err != nil {
		return nil, fmt.Errorf("cannot load template: %w", err)
	}

	funcMap := template.FuncMap{
		"quote": func(s string) string { return strconv.Quote(s) },
	}

	tmpl, err := template.New("template").Funcs(funcMap).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("error parsing template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("error executing template: %v", err)
	}

	return buf.Bytes(), nil
}
