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
	"net"
	"strconv"
	"text/template"
	"unicode/utf8"
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
		"quote":    yamlQuote,
		"hostPort": hostPort,
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

// yamlQuote renders a value as a YAML double-quoted scalar.
//
// Every value substituted into a template must go through this function. The
// rendered files are read back by kubelet, distribution, docker_auth and
// mirrorer; a value able to close its scalar and open a new mapping key changes
// the meaning of the file rather than a field in it, and the static pod manifest
// in particular is executed as root on a control plane node.
//
// strconv.Quote produces a Go string literal, which is a valid YAML
// double-quoted scalar for every input but one: Go writes invalid UTF-8 as
// \xNN, while YAML reads \xNN as the code point U+00NN. Such a value has no YAML
// representation at all, so it is rejected instead of being silently changed --
// an account name that renders as a different name would leave the auth
// configuration and the distributed credentials disagreeing.
func yamlQuote(value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("value is not valid UTF-8 and has no YAML representation: %q", value)
	}
	return strconv.Quote(value), nil
}

// hostPort joins a host and a port for use in a configuration value. It brackets
// IPv6 addresses, which plain concatenation would render ambiguous.
func hostPort(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
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
