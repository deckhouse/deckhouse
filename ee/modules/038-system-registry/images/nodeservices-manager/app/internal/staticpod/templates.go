/*
Copyright 2025 Flant JSC

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
