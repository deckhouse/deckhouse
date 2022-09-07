package internal

import (
	"bytes"
	"text/template"
)

func TemplateToYAML(tmpl string, params interface{}) string {
	var output bytes.Buffer
	t := template.Must(template.New("").Parse(tmpl))
	_ = t.Execute(&output, params)
	return output.String()
}
