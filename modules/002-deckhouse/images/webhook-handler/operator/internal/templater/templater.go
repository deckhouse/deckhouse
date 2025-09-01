package templater

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	deckhouseiov1alpha1 "deckhouse.io/webhook/api/v1alpha1"
	"sigs.k8s.io/yaml"
)

func RenderTemplate(tpl string, vh *deckhouseiov1alpha1.ValidationWebhook) (*bytes.Buffer, error) {
	tplt, err := template.New("test").Funcs(template.FuncMap{
		"toYaml": toYAML,
		"indent": indent,
		"list":   list,
	}).Parse(tpl)
	if err != nil {
		return nil, fmt.Errorf("template parse: %w", err)
	}

	var buf bytes.Buffer

	err = tplt.Execute(&buf, vh)
	if err != nil {
		return nil, fmt.Errorf("template execute: %w", err)
	}

	// debug
	// log.Info("template", slog.String("template", buf.String()))
	fmt.Println(buf.String())

	return &buf, nil
}

// toYAML takes an interface, marshals it to yaml, and returns a string. It will
// always return a string, even on marshal error (empty string).
//
// This is designed to be called from a template.
func toYAML(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}

	data, err = yaml.JSONToYAML(data)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}

	return strings.TrimSuffix(string(data), "\n")
}

func indent(spaces int, s string) string {
	pad := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}

func list(objs ...any) []any {
	return objs
}
