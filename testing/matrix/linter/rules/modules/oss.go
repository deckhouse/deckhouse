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
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	linterrors "github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
)

const ossFilename = "oss.yaml"

func ossModuleRule(name, moduleRoot string) linterrors.LintRuleErrorsList {
	lintErrors := linterrors.LintRuleErrorsList{}

	if errs := verifyOssFile(name, moduleRoot); len(errs) > 0 {
		for _, err := range errs {
			ruleErr := linterrors.NewLintRuleError(
				"MODULE001",
				moduleLabel(name),
				nil,
				ossFileErrorMessage(err),
			)

			lintErrors.Add(ruleErr)
		}
	}

	return lintErrors
}

func ossFileErrorMessage(err error) string {
	if os.IsNotExist(err) {
		return "Module should have " + ossFilename
	}
	return fmt.Sprintf("Invalid %s: %s", ossFilename, err.Error())
}

func verifyOssFile(name, moduleRoot string) []error {
	if shouldIgnoreOssInfo(name) {
		return nil
	}

	projects, err := readOssFile(moduleRoot)
	if err == nil && len(projects) == 0 {
		err = fmt.Errorf("no projects described")
	}
	if err != nil {
		return []error{err}
	}

	var errs []error
	for i, p := range projects {
		err := assertOssProject(i+1, p)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func assertOssProject(i int, p ossProject) error {
	var complaints []string

	// prefix to make it easier navigate among errors
	prefix := fmt.Sprintf("#%d", i)

	// Name

	if strings.TrimSpace(p.Name) == "" {
		complaints = append(complaints, "name must not be empty")
	} else {
		prefix = fmt.Sprintf("#%d (name=%s)", i, p.Name)
	}

	// Description

	if strings.TrimSpace(p.Description) == "" {
		complaints = append(complaints, "description must not be empty")
	}

	// Link

	if strings.TrimSpace(p.Link) == "" {
		complaints = append(complaints, "link must not be empty")
	} else if _, err := url.ParseRequestURI(p.Link); err != nil {
		complaints = append(complaints, fmt.Sprintf("link URL is malformed (%q)", p.Link))
	}

	// Licence

	if strings.TrimSpace(p.Licence) == "" {
		complaints = append(complaints, "licence must not be empty")
	}

	// Logo

	if strings.TrimSpace(p.Logo) != "" {
		if _, err := url.ParseRequestURI(p.Logo); err != nil {
			complaints = append(complaints, fmt.Sprintf("project logo URL is malformed (%q)", p.Logo))
		}
	}

	if len(complaints) > 0 {
		return fmt.Errorf("%s: %s", prefix, strings.Join(complaints, "; "))
	}

	return nil
}

func readOssFile(moduleRoot string) ([]ossProject, error) {
	b, err := os.ReadFile(filepath.Join(moduleRoot, ossFilename))
	if err != nil {
		return nil, err
	}

	return parseProjectList(b)
}

func parseProjectList(b []byte) ([]ossProject, error) {
	var projects []ossProject
	err := yaml.Unmarshal(b, &projects)
	if err != nil {
		return nil, err
	}
	return projects, nil
}

var skipOssChecks = map[string]struct{}{
	// module name
	"001-priority-class":                      {},
	"005-external-module-manager":             {},
	"039-registry-packages-proxy":             {},
	"011-flow-schema":                         {},
	"013-helm":                                {}, // helm in 002-deckhouse
	"036-kube-proxy":                          {},
	"030-cloud-provider-aws":                  {},
	"030-cloud-provider-azure":                {},
	"030-cloud-provider-gcp":                  {},
	"030-cloud-provider-openstack":            {},
	"030-cloud-provider-vsphere":              {},
	"030-cloud-provider-vcd":                  {},
	"030-cloud-provider-yandex":               {},
	"030-cloud-provider-zvirt":                {},
	"035-cni-simple-bridge":                   {},
	"140-user-authz":                          {},
	"160-multitenancy-manager":                {},
	"340-extended-monitoring":                 {},
	"340-monitoring-applications":             {},
	"340-monitoring-custom":                   {},
	"340-monitoring-deckhouse":                {},
	"340-monitoring-kubernetes-control-plane": {},
	"340-monitoring-ping":                     {},
	"350-node-local-dns":                      {},
	"400-nginx-ingress":                       {}, // nginx in 402-ingress-nginx
	"450-network-gateway":                     {},
	"500-basic-auth":                          {}, // nginx in 402-ingress-nginx
	"500-okmeter":                             {},
	"500-upmeter":                             {},
	"600-secret-copier":                       {},
	"810-documentation":                       {},
}

// TODO When lintignore files will be implemented in modules, detect "oss.yaml" line in it
func shouldIgnoreOssInfo(moduleName string) bool {
	_, found := skipOssChecks[moduleName]
	return found
}

type ossProject struct {
	Name        string `yaml:"name"`           // example: Dex
	Description string `yaml:"description"`    // example: A Federated OpenID Connect Provider with pluggable connectors
	Link        string `yaml:"link"`           // example: https://github.com/dexidp/dex
	Logo        string `yaml:"logo,omitempty"` // example: https://dexidp.io/img/logos/dex-horizontal-color.png
	Licence     string `yaml:"licence"`        // example: Apache License 2.0
}
