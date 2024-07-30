/*
Copyright 2021 Flant JSC

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

package modules

import (
	"fmt"
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

func shouldSkipCrd(name string) bool {
	return !strings.Contains(name, "deckhouse.io")
}

func crdsModuleRule(name, path string) errors.LintRuleErrorsList {
	var lintRuleErrorsList errors.LintRuleErrorsList
	_ = filepath.Walk(path, func(path string, _ os.FileInfo, _ error) error {
		if filepath.Ext(path) != ".yaml" {
			return nil
		}

		fileContent, err := os.ReadFile(path)
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
					"Can't parse manifests in %s folder", crdsDir,
				))
			}

			if shouldSkipCrd(crd.Name) {
				continue
			}

			if crd.APIVersion != "apiextensions.k8s.io/v1" {
				lintRuleErrorsList.Add(errors.NewLintRuleError(
					"MODULE004",
					fmt.Sprintf("kind = %s ; name = %s ; module = %s ; file = %s", crd.Kind, crd.Name, name, path),
					crd.APIVersion,
					"CRD specified using deprecated api version, wanted \"apiextensions.k8s.io/v1\"",
				))
			}
		}
		return nil
	})
	return lintRuleErrorsList
}
