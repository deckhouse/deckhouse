package template

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"text/template"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/state"

	dhctltemp "github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type State interface {
	ConvertToMap() (map[string]interface{}, error)
	GetClusterType() string
	GetRegistryState() state.RegistryState
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

	r, err := generateDockerConfig(b.GetRegistryState())
	if err != nil {
		return "", err
	}

	data["registry_state"].(map[string]interface{})["dockerconf"] = r

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

func generateDockerConfig(r state.RegistryState) (string, error) {
	u, err := url.Parse("http://" + r.Repo)
	if err != nil {
		return "", err
	}

	type auth struct {
		Auth string `json:"auth,omitempty"`
	}

	type dockerConfig struct {
		Auths map[string]auth `json:"auths"`
	}

	a := auth{}
	if r.User != "" && r.Password != "" {
		a.Auth = base64.StdEncoding.EncodeToString([]byte(r.User + ":" + r.Password))
	}

	c := dockerConfig{
		Auths: map[string]auth{
			u.Host: a,
		},
	}

	jsonData, err := json.Marshal(c)
	if err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(jsonData)

	return encoded, nil
}
