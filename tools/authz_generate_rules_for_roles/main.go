/*
Copyright 2023 Flant JSC

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

/*
This binary is used for generating rules for user-authz roles to
./modules/140-user-authz/docs/README.md and ./modules/140-user-authz/docs/README_RU.md.
It inserts data between lines "<!-- start user-authz roles placeholder -->" and "<!-- end user-authz roles placeholder -->".
It useses rendered template from /deckhouse/modules/140-user-authz/templates/cluster-roles.yaml
Steps to use:
  - cd tools && go generate
  - cd ../ && make lint-markdown-fix
  - check diff for ./modules/140-user-authz/docs/README.md and ./modules/140-user-authz/docs/README_RU.md files
*/

package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/sprig"
	"github.com/deckhouse/deckhouse/testing/library/helm"
	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v3"
)

var (
	readVerbs      = sliceToString([]string{"get", "list", "watch"})
	writeVerbs     = sliceToString([]string{"create", "delete", "deletecollection", "patch", "update"})
	readWriteVerbs = sliceToString([]string{"get", "list", "watch", "create", "delete", "deletecollection", "patch", "update"})

	readAlias      = "read"
	writeAlias     = "write"
	readWriteAlias = "read-write"

	userRole           = "User"
	privilegedUserRole = "PrivilegedUser"
	editorRole         = "Editor"
	adminRole          = "Admin"
	clusterEditorRole  = "ClusterEditor"
	clusterAdminRole   = "ClusterAdmin"

	orderedRoleNames = []string{
		userRole,
		privilegedUserRole,
		editorRole,
		adminRole,
		clusterEditorRole,
		clusterAdminRole,
	}

	neededClusterRoleExcludes = map[string][]string{
		userRole:           {},
		privilegedUserRole: {userRole},
		editorRole:         {userRole, privilegedUserRole},
		adminRole:          {userRole, privilegedUserRole, editorRole},
		clusterEditorRole:  {userRole, privilegedUserRole, editorRole},
		clusterAdminRole:   {userRole, privilegedUserRole, editorRole, adminRole, clusterEditorRole},
	}
)

type TemplateData struct {
	Roles []TemplateRole
}

func (t *TemplateData) toValues() map[string]interface{} {
	return map[string]interface{}{"roles": t.Roles}
}

type TemplateRole struct {
	Name            string
	Rules           map[string][]string
	AdditionalRoles []string
}

const readmeTemplate = "* read - `get`, `list`, `watch`\n" +
	"* read-write - `get`, `list`, `watch`, `create`, `delete`, `deletecollection`, `patch`, `update`\n" +
	"* write - `create`, `delete`, `deletecollection`, `patch`, `update`\n\n" +
	"```yaml\n" +
	"{{range $role := .roles}}" +
	"Role `{{$role.Name}}`{{if $role.AdditionalRoles}}{{printf \" (and all rules from `%s`)\" ($role.AdditionalRoles | join \"`, `\")}}{{end}}:\n" +
	"{{$role.Rules | toYaml | indent 4 }}\n" +
	"{{end}}" +
	"```\n"

func main() {
	renderContents, err := renderTemplates("../modules/140-user-authz")
	if err != nil {
		log.Fatalln(err)
	}

	clusterRolesMap := getClusterRoles(renderContents, "user-authz/templates/cluster-roles.yaml")
	if err != nil {
		log.Fatalln(err)
	}

	// map[<role name>]map[<verb>][]<resources>
	crVerbResourceMap := make(map[string]map[string][]string)
	for name, rules := range clusterRolesMap {
		if _, f := neededClusterRoleExcludes[name]; !f {
			continue
		}
		crVerbResourceMap[name] = processClusterRoleRules(rules)
	}
	templateData := TemplateData{}
	for _, name := range orderedRoleNames {
		templateData.Roles = append(templateData.Roles, prepareClusterRoleForTemplate(name, crVerbResourceMap))
	}

	readmeContent, err := renderTemplate(readmeTemplate, templateData)
	if err != nil {
		log.Fatalln(err)
	}

	for _, fileName := range os.Args[1:] {
		if err := updateReadme(fileName, readmeContent); err != nil {
			log.Fatalln(err)
		}
	}
}

func renderTemplates(dir string) (map[string]string, error) {
	r := helm.Renderer{}
	return r.RenderChartFromDir(dir, `{"userAuthz":{"internal":{}},"global":{}}`)
}

type clusterRole struct {
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Rules []rule `yaml:"rules"`
}

type rule struct {
	Verbs     []string `yaml:"verbs"`
	APIGroups []string `yaml:"apiGroups"`
	Resources []string `yaml:"resources"`
}

func getClusterRoles(contents map[string]string, fileName string) map[string][]rule {
	dec := yaml.NewDecoder(strings.NewReader(contents[fileName]))
	roleMap := make(map[string][]rule)
	for {
		var r clusterRole
		if err := dec.Decode(&r); err != nil || len(r.Rules) < 1 {
			if errors.Is(err, io.EOF) {
				return roleMap
			}
			continue
		}
		roleName := newRoleFromClusterRoleName(r.Metadata.Name)
		roleMap[roleName] = r.Rules
	}
}

func processClusterRoleRules(rules []rule) map[string][]string {
	rulesMap := make(map[string][]string)
	for _, r := range rules {
		verbAlias, resources := processRule(r)
		rulesMap[verbAlias] = append(rulesMap[verbAlias], resources...)
	}
	return rulesMap
}

func processRule(r rule) (string, []string) {
	grSlice := make([]string, 0, len(r.APIGroups)*len(r.Resources))
	for _, apiGroup := range r.APIGroups {
		if apiGroup != "" {
			apiGroup = apiGroup + "/"
		}
		for _, resource := range r.Resources {
			grSlice = append(grSlice, apiGroup+resource)
		}
	}
	alias := sliceToString(r.Verbs)
	switch alias {
	case readVerbs:
		alias = readAlias
	case writeVerbs:
		alias = writeAlias
	case readWriteVerbs:
		alias = readWriteAlias
	}
	return alias, grSlice
}

func prepareClusterRoleForTemplate(name string, crVerbResourceMap map[string]map[string][]string) TemplateRole {
	templateRole := TemplateRole{Name: name}
	excls := neededClusterRoleExcludes[name]
	if len(excls) > 0 {
		templateRole.AdditionalRoles = excls
	}

	excludesMap := clusterRoleGenerateExcludes(excls, crVerbResourceMap)
	templateRole.Rules = clusterRoleApplyExcludes(crVerbResourceMap[name], excludesMap)
	return templateRole
}

func clusterRoleGenerateExcludes(excludeNames []string, rulesMap map[string]map[string][]string) map[string]map[string]struct{} {
	excludesMap := make(map[string]map[string]struct{})
	for _, name := range excludeNames {
		for verb, resources := range rulesMap[name] {
			if excludesMap[verb] == nil {
				excludesMap[verb] = make(map[string]struct{})
			}
			for _, resource := range resources {
				excludesMap[verb][resource] = struct{}{}
			}
		}
	}
	return excludesMap
}

func clusterRoleApplyExcludes(roleRules map[string][]string, excludesMap map[string]map[string]struct{}) map[string][]string {
	resultMap := make(map[string][]string, len(roleRules))
	for verb, resources := range roleRules {
		resultResources := make([]string, 0)
		for _, resource := range resources {
			if _, f := excludesMap[verb][resource]; f {
				continue
			}
			resultResources = append(resultResources, resource)
		}
		if len(resultResources) < 1 {
			delete(resultMap, verb)
			continue
		}
		resultMap[verb] = sortStrings(resultResources)
	}
	return resultMap
}

func updateReadme(filePath string, content []byte) error {
	const (
		startPlaceholder = "<!-- start user-authz roles placeholder -->"
		endPlaceholder   = "<!-- end user-authz roles placeholder -->"
	)

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	fileText, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	newFileContents, err := replacePlaceholder(fileText, content, startPlaceholder, endPlaceholder)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, []byte(newFileContents), 0644); err != nil {
		return err
	}
	return nil
}

func renderTemplate(templateString string, tempalteData TemplateData) ([]byte, error) {
	var res bytes.Buffer
	templateFuncMap := sprig.TxtFuncMap()
	templateFuncMap["toYaml"] = tempalteToYaml

	tpl, err := template.New("template").
		Funcs(templateFuncMap).
		Parse(templateString)
	if err != nil {
		return nil, err
	}

	if err := tpl.Execute(&res, tempalteData.toValues()); err != nil {
		return nil, err
	}

	return bytes.TrimSpace(res.Bytes()), nil
}

func replacePlaceholder(text, content []byte, startPlaceholder, endPlaceholder string) ([]byte, error) {
	re, err := regexp.Compile(fmt.Sprintf("(?s)%s(.*?)%s", startPlaceholder, endPlaceholder))
	if err != nil {
		return nil, err
	}
	subMatchIndexes := re.FindSubmatchIndex(text)
	if len(subMatchIndexes) < 4 {
		return nil, fmt.Errorf("didn't find submatch inside placeholder `%s` and `%s`", startPlaceholder, endPlaceholder)
	}

	placeholderStart := subMatchIndexes[2] + 1
	placeHolderEnd := subMatchIndexes[3] - 1

	buf := bytes.NewBuffer(nil)
	buf.Write(text[:placeholderStart])
	buf.Write(content)
	buf.Write(text[placeHolderEnd:])
	return buf.Bytes(), nil
}

// tempalteToYaml is a template function that encodes an item into a Yaml string
func tempalteToYaml(v interface{}) string {
	output, _ := yaml.Marshal(v)
	return string(output)
}

// newRoleFromClusterRoleName generates user-authz role name from ClusterRole name
func newRoleFromClusterRoleName(name string) string {
	return strcase.ToCamel(strings.TrimPrefix(name, "user-authz:"))
}

func sliceToString(s []string) string {
	return strings.Join(sortStrings(s), ",")
}

func sortStrings(s []string) []string {
	rs := make([]string, len(s))
	copy(rs, s)
	sort.Strings(rs)
	return rs
}
