package crds

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

//go:embed *.yaml
var FS embed.FS

func List() ([]apiextensionsv1.CustomResourceDefinition, error) {
	var result []apiextensionsv1.CustomResourceDefinition
	return result, fs.WalkDir(FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("fs io error: %w", err)
		}

		if d.IsDir() {
			return nil
		}

		if strings.HasPrefix(path, "doc-ru-") {
			return nil
		}

		rawData, err := FS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		var crd apiextensionsv1.CustomResourceDefinition
		err = yaml.Unmarshal(rawData, &crd)
		if err != nil {
			return fmt.Errorf("unmarshal crd: %w", err)
		}

		result = append(result, crd)
		return nil
	})
}
