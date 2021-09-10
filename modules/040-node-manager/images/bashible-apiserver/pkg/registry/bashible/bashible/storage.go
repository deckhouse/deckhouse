package bashible

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"d8.io/bashible/pkg/apis/bashible"
	"d8.io/bashible/pkg/template"
)

const templateName = "bashible.sh.tpl"

// NewStorage returns storage object that will work against API services.
func NewStorage(rootDir string, bashibleContext template.Context) (*Storage, error) {
	templatePath := path.Join(rootDir, "bashible", templateName)

	tplContent, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read template: %v", err)
	}

	storage := &Storage{
		templateContent: tplContent,
		templateName:    templateName,
		bashibleContext: bashibleContext,
	}

	return storage, nil
}

type Storage struct {
	templateContent []byte
	templateName    string
	bashibleContext template.Context
}

// Render renders single script content by name
func (s Storage) Render(name string) (runtime.Object, error) {
	data, err := s.getContext(name)
	if err != nil {
		return nil, fmt.Errorf("cannot get context: %v", err)
	}
	r, err := template.RenderTemplate(templateName, s.templateContent, data)
	if err != nil {
		return nil, fmt.Errorf("cannot render template: %v", err)
	}

	obj := bashible.Bashible{}
	obj.ObjectMeta.Name = name
	obj.ObjectMeta.CreationTimestamp = metav1.NewTime(time.Now())
	obj.Data = map[string]string{}
	obj.Data[r.FileName] = r.Content.String()

	return &obj, nil
}

func (s Storage) getContext(name string) (map[string]interface{}, error) {
	contextKey, err := template.GetBashibleContextKey(name)
	if err != nil {
		return nil, fmt.Errorf("cannot get context key: %v", err)
	}

	context, err := s.bashibleContext.Get(contextKey)
	if err != nil {
		return nil, fmt.Errorf("cannot get context data: %v", err)
	}

	err = s.enrichContext(context)
	if err != nil {
		return nil, fmt.Errorf("cannot get registry context data: %v", err)
	}

	return context, nil
}

func (s Storage) New() runtime.Object {
	return &bashible.Bashible{}
}

func (s Storage) NewList() runtime.Object {
	return &bashible.BashibleList{}
}

func (s Storage) enrichContext(context map[string]interface{}) error {
	// enrich context with registry path and dockerCfg
	type dockerCfg struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}

	var (
		registryHost string
		registryAuth string
		dc           dockerCfg
	)

	registryMapContext, err := s.bashibleContext.Get("registry")
	if err != nil {
		return fmt.Errorf("cannot get registry context data: %v", err)
	}

	registryPath, ok := registryMapContext["path"]
	if !ok {
		return fmt.Errorf("cannot get path from registry context: %v", registryMapContext["path"])
	}
	registryHost = strings.Split(registryPath.(string), "/")[0]

	if registryDockerCfgJSONBase64, ok := registryMapContext["dockerCfg"]; ok {
		bytes, err := base64.StdEncoding.DecodeString(registryDockerCfgJSONBase64.(string))
		if err != nil {
			return fmt.Errorf("cannot base64 decode docker cfg: %v", err)
		}

		err = json.Unmarshal(bytes, &dc)
		if err != nil {
			return fmt.Errorf("cannot unmarshal docker cfg: %v", err)
		}

		if registry, ok := dc.Auths[registryHost]; ok {
			bytes, err = base64.StdEncoding.DecodeString(registry.Auth)
			if err != nil {
				return fmt.Errorf("cannot base64 decode auth string: %v", err)
			}
			registryAuth = string(bytes)
		}
	}

	context["registry"] = map[string]interface{}{"host": registryHost, "auth": registryAuth}
	return nil
}
