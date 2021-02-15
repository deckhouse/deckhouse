package template

import (
	"bytes"
	"strings"
)

type RenderedTemplate struct {
	Content  *bytes.Buffer
	FileName string
}

// RenderTemplate renders the template using the wrapper knowing the template name, its body and the template data to fill in.
func RenderTemplate(name string, content []byte, data map[string]interface{}) (*RenderedTemplate, error) {
	e := Engine{
		Name: name,
		Data: data,
	}

	out, err := e.Render(content)
	if err != nil {
		return nil, err
	}

	rendered := &RenderedTemplate{
		Content:  out,
		FileName: strings.TrimSuffix(name, ".tpl"),
	}

	return rendered, nil
}
