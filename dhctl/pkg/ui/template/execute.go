package template

import (
	"bytes"
	"text/template"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/state"

	dhctltemp "github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type State interface {
	ConvertToMap() (map[string]interface{}, error)
	GetClusterType() string
}

func RenderTemplate(b State) (string, error) {
	data, err := b.ConvertToMap()
	if err != nil {
		return "", err
	}

	content := ""

	if b.GetClusterType() == state.StaticCluster {
		content = staticTemplate
	}

	t := template.New("resource_render").Funcs(dhctltemp.FuncMap())
	t, err = t.Parse(content)
	if err != nil {
		return "", err
	}

	var tpl bytes.Buffer

	err = t.Execute(&tpl, data)
	if err != nil {
		return "", err
	}

	return tpl.String(), nil
}
