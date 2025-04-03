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

// RenderTemplate renders the provided template content with the given data
func renderTemplate(name string, data interface{}) ([]byte, error) {
	content, err := templatesFS.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("cannot load template: %w", err)
	}

	funcMap := template.FuncMap{
		"quote": strconv.Quote,
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

type templateRenderer interface {
	Render() ([]byte, error)
}

// processTemplate processes the given template file and saves the rendered result to the specified path
func processTemplate(renderer templateRenderer, outputPath string) (bool, string, error) {
	// Render the template with the given configuration
	renderedContent, err := renderer.Render()
	if err != nil {
		return false, "", fmt.Errorf("failed to render template %w", err)
	}

	chaged, hash, err := saveFileIfChanged(outputPath, renderedContent)
	if err != nil {
		return chaged, hash, fmt.Errorf("failed to save file %s: %w", outputPath, err)
	}
	return chaged, hash, nil
}
