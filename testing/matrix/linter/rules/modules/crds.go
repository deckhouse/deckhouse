package modules

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
)

var (
	sep = regexp.MustCompile("(?:^|\\s*\n)---\\s*")
)

func crdsModuleRule(name, path string) errors.LintRuleErrorsList {
	var lintRuleErrorsList errors.LintRuleErrorsList

	_ = filepath.Walk(path, func(path string, info os.FileInfo, _ error) error {
		if filepath.Ext(path) != ".yaml" {
			return nil
		}

		fileContent, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		bigFileTmp := strings.TrimSpace(string(fileContent))
		docs := sep.Split(bigFileTmp, -1)
		for _, d := range docs {
			if d == "" {
				continue
			}

			d = strings.TrimSpace(d)
			var crd v1beta1.CustomResourceDefinition

			err = yaml.Unmarshal([]byte(d), &crd)
			if err != nil {
				lintRuleErrorsList.Add(errors.NewLintRuleError(
					"MODULE004",
					"module = "+name,
					err.Error(),
					"Can't parse manifests in crds folder",
				))
			}

			// Enable this after all clusters will be upgraded to 1.16+
			/*
				if crd.APIVersion != "apiextensions.k8s.io/v1" {
					lintRuleErrorsList.Add(errors.NewLintRuleError(
						"MODULE004",
						fmt.Sprintf("kind = %s ; name = %s ; module = %s", crd.Kind, crd.Name, name),
						crd.APIVersion,
						"CRD specified using deprecated api version, wanted \"apiextensions.k8s.io/v1\"",
					))
				}
			*/
		}
		return nil
	})
	return lintRuleErrorsList
}
