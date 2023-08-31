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
../modules/140-user-authz/docs/README.md and ../modules/140-user-authz/docs/README_RU.md.
It inserts data between lines "<!-- start user-authz roles placeholder -->" and "<!-- end user-authz roles placeholder -->".
It useses rendered template rendering by "github.com/deckhouse/deckhouse/testing/library/helm" lib
Steps to use:
  - make generate
  - make lint-markdown-fix
  - check diff for ./modules/140-user-authz/docs/README.md and ./modules/140-user-authz/docs/README_RU.md files
*/

package authzgeneraterulesforroles

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"tools/helm_generate/helper"

	"github.com/Masterminds/sprig"
	"github.com/deckhouse/deckhouse/testing/library/helm"
	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v3"
)

var (
	userRole           = "User"
	privilegedUserRole = "PrivilegedUser"
	editorRole         = "Editor"
	adminRole          = "Admin"
	clusterEditorRole  = "ClusterEditor"
	clusterAdminRole   = "ClusterAdmin"

	// Predefined order for roles for printed doc
	orderedRoleNames = []string{
		userRole,
		privilegedUserRole,
		editorRole,
		adminRole,
		clusterEditorRole,
		clusterAdminRole,
	}

	// Excludes for roles (refer to ../modules/user-authz/templates/cluster-roles.yaml)
	// And this map is also used for filtering target cluster roles
	neededClusterRoleExcludes = map[string][]string{
		userRole:           {},
		privilegedUserRole: {userRole},
		editorRole:         {userRole, privilegedUserRole},
		adminRole:          {userRole, privilegedUserRole, editorRole},
		clusterEditorRole:  {userRole, privilegedUserRole, editorRole},
		clusterAdminRole:   {userRole, privilegedUserRole, editorRole, adminRole, clusterEditorRole},
	}
)

// readmeTemplateData is data for readme template
type readmeTemplateData struct {
	Roles   []role  `json:"roles"`
	Aliases []alias `json:"aliases"`
}

// toValues converts readmeTemplateData to values for templates
// (implements "templateData" interface)
func (t *readmeTemplateData) toValues() map[string]interface{} {
	var res map[string]interface{}
	marshal, _ := json.Marshal(t)
	_ = json.Unmarshal(marshal, &res)
	return res
}

// role is representation of ClusterRole verbs for readme template
type role struct {
	Name            string              `json:"name"`
	Rules           map[string][]string `json:"rules"`
	AdditionalRoles []string            `json:"additionalRoles"`
}

// alias is representation of commonly used verbs group for readme template
type alias struct {
	Name  string   `json:"name"`
	Verbs []string `json:"verbs"`
	// verbsJoined represents "Verbs" joined with comma
	// (refer to newAlias)
	verbsJoined string `json:"-"`
}

func newAlias(name string, verbs []string) alias {
	return alias{
		Name:        name,
		Verbs:       verbs,
		verbsJoined: sliceToString(verbs),
	}
}

// aliases for verbs commonly used in user-authz ClusterRole templates
var (
	readAlias = newAlias(
		"read",
		[]string{"get", "list", "watch"},
	)
	writeAlias = newAlias(
		"write",
		[]string{"create", "delete", "deletecollection", "patch", "update"},
	)
	readWriteAlias = newAlias(
		"read-write",
		[]string{"get", "list", "watch", "create", "delete", "deletecollection", "patch", "update"},
	)

	aliases = []alias{
		readAlias,
		readWriteAlias,
		writeAlias,
	}
)

// isAliased checks that verbs string has an alias
func isAliased(verbs string) (string, bool) {
	for _, al := range aliases {
		if al.verbsJoined == verbs {
			return al.Name, true
		}
	}
	return "", false
}

func run() {
	// "github.com/deckhouse/deckhouse/testing/library" InitValues() can be used to seed render values.
	renderContents, err := renderHelmTemplates("../modules/140-user-authz", `{"userAuthz":{"internal":{}},"global":{}}`)
	if err != nil {
		log.Fatalln(err)
	}

	clusterRolesMap := getClusterRoles(renderContents, "user-authz/templates/cluster-roles.yaml")
	if err != nil {
		log.Fatalln(err)
	}

	// map[<role name>]map[<verb>][]<resource>
	crVerbResourceMap := make(map[string]map[string][]string, len(neededClusterRoleExcludes))
	for name, rules := range clusterRolesMap {
		if _, f := neededClusterRoleExcludes[name]; !f {
			continue
		}
		crVerbResourceMap[name] = processClusterRoleRules(rules)
	}

	templateData := &readmeTemplateData{
		Aliases: aliases,
	}
	for _, roleName := range orderedRoleNames {
		templateData.Roles = append(templateData.Roles, prepareClusterRoleForTemplate(roleName, neededClusterRoleExcludes[roleName], crVerbResourceMap))
	}

	readmeContent, err := renderTemplate("helm_generate/runners/authz_generate_rules_for_roles/readme-placeholder.tpl", templateData)
	if err != nil {
		log.Fatalln(err)
	}

	readmeFiles := []string{
		"../modules/140-user-authz/docs/README.md",
		"../modules/140-user-authz/docs/README_RU.md",
	}
	for _, fileName := range readmeFiles {
		if err := updateReadme(fileName, readmeContent); err != nil {
			log.Fatalln(err)
		}
	}
}

// renderHelmTemplates renders helm template for chart directory "dir" with values "values"
func renderHelmTemplates(dir, values string) (map[string]string, error) {
	deckhouseRoot, err := helper.DeckhouseRoot()
	if err != nil {
		return nil, err
	}

	helmLibPath := "helm_lib/charts/deckhouse_lib_helm"
	chartHelmLibPath := filepath.Join(deckhouseRoot, "modules/140-user-authz/charts/helm_lib")
	if err := os.Remove(chartHelmLibPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	helmLibFullPath := filepath.Join(deckhouseRoot, helmLibPath)
	if err := os.Symlink(helmLibFullPath, chartHelmLibPath); err != nil {
		return nil, err
	}

	defer func() {
		_ = os.Remove(chartHelmLibPath)
		helmLibFullPath = filepath.Join("/deckhouse", helmLibPath)
		_ = os.Symlink(helmLibFullPath, chartHelmLibPath)
	}()

	if err := os.Chdir(filepath.Join(deckhouseRoot, "tools")); err != nil {
		return nil, err
	}

	r := helm.Renderer{}
	return r.RenderChartFromDir(dir, values)
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

// getClusterRoles retrieves ClusterRole template file "fileName" from helm templates map "templates"
// and converts ClusterRoles verbs and resources to map
func getClusterRoles(templates map[string]string, fileName string) map[string][]rule /*map[<role name>][]<role rules>*/ {
	dec := yaml.NewDecoder(strings.NewReader(templates[fileName]))
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

// processClusterRoleRules generates map of verbs with resources lists for rules list
func processClusterRoleRules(rules []rule) map[string][]string /*map[<verb>][]<resource>*/ {
	rulesMap := make(map[string][]string)
	for _, r := range rules {
		verbAlias, resources := processRule(r)
		rulesMap[verbAlias] = append(rulesMap[verbAlias], resources...)
	}
	return rulesMap
}

// processRule retrieves verbs names and resources list from rule
func processRule(r rule) (string /*<verbs or verbs alias>*/, []string /*[]<resource>*/) {
	grSlice := make([]string, 0, len(r.APIGroups)*len(r.Resources))
	for _, apiGroup := range r.APIGroups {
		if apiGroup != "" {
			apiGroup = apiGroup + "/"
		}
		for _, resource := range r.Resources {
			grSlice = append(grSlice, apiGroup+resource)
		}
	}

	verbsJoined := sliceToString(r.Verbs)
	if al, f := isAliased(verbsJoined); f {
		verbsJoined = al
	}

	return verbsJoined, grSlice
}

// prepareClusterRoleForTemplate generates template data for ClusterRole
func prepareClusterRoleForTemplate(roleName string, excls []string, crVerbResourceMap map[string]map[string][]string /*map[<role name>]map[<verb>][]<resource>*/) role {
	templateRole := role{Name: roleName}

	var excludesMap map[string]map[string]struct{}
	if len(excls) > 0 {
		templateRole.AdditionalRoles = excls
		excludesMap = clusterRoleGenerateExcludes(excls, crVerbResourceMap)
	}

	templateRole.Rules = clusterRoleApplyExcludes(crVerbResourceMap[roleName], excludesMap)
	return templateRole
}

// clusterRoleGenerateExcludes generates map with verbs and resources for excludes
func clusterRoleGenerateExcludes(excludeRoleNames []string, rulesMap map[string]map[string][]string /*map[<role name>]map[<verb>][]<resource>*/) map[string]map[string]struct{} /*map[<verb>]map[<resource>]{}*/ {
	if len(excludeRoleNames) < 1 {
		return nil
	}

	excludesMap := make(map[string]map[string]struct{})
	for _, name := range excludeRoleNames {
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

// clusterRoleApplyExcludes removes from "roleRules" verbs resources already existed in "excludesMap"
func clusterRoleApplyExcludes(roleRules map[string][]string /*map[<verb>][]<resource>*/, excludesMap map[string]map[string]struct{} /*map[<verb>]map[<resource>]{}*/) map[string][]string /*map[<verb>][]<resource>*/ {
	resultMap := make(map[string][]string, len(roleRules))
	for verb, resources := range roleRules {
		resultResources := make([]string, 0)
		for _, resource := range resources {
			if verbMap, f := excludesMap[verb]; f {
				if _, f := verbMap[resource]; f {
					continue
				}
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

// updateReadme opens "fileName" file and replaces it's contents
// between "<!-- start user-authz roles placeholder -->" and
// "<!-- end user-authz roles placeholder -->" with "content"
func updateReadme(fileName string, content []byte) error {
	const (
		startPlaceholder = "<!-- start user-authz roles placeholder -->"
		endPlaceholder   = "<!-- end user-authz roles placeholder -->"
	)

	f, err := os.Open(fileName)
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

	if err := os.WriteFile(fileName, []byte(newFileContents), 0644); err != nil {
		return err
	}
	return nil
}

// templateData is an interface for rendering tempaltes
type templateData interface {
	toValues() map[string]interface{}
}

// renderTemplate renders template from "templateFile" file with values from templateData interface{}
func renderTemplate(templateFile string, tempalteData templateData) ([]byte, error) {
	templateFuncMap := sprig.TxtFuncMap()

	// template function that encodes an item into a Yaml string
	templateFuncMap["toYaml"] = func(v interface{}) string {
		output, _ := yaml.Marshal(v)
		return string(output)
	}
	tpl, err := template.New(filepath.Base(templateFile)).
		Funcs(templateFuncMap).
		ParseFiles(templateFile)
	if err != nil {
		return nil, err
	}

	var res bytes.Buffer
	if err := tpl.Execute(&res, tempalteData.toValues()); err != nil {
		return nil, err
	}

	return bytes.TrimSpace(res.Bytes()), nil
}

// replacePlaceholder replaces contents in "text" between "startPlaceholder" and "endPlaceholder" with "replaceContent"
func replacePlaceholder(text, replaceContent []byte, startPlaceholder, endPlaceholder string) ([]byte, error) {
	// refer to https://github.com/google/re2/wiki/Syntax
	re, err := regexp.Compile(fmt.Sprintf("(?s)%s(.*?)%s", startPlaceholder, endPlaceholder))
	if err != nil {
		return nil, err
	}

	// Find submatch for first subexpression (refer to re.FindSubmatchIndex doc)
	subMatchIndexes := re.FindSubmatchIndex(text)
	if len(subMatchIndexes) < 4 {
		return nil, fmt.Errorf("didn't find submatch inside placeholder `%s` and `%s`", startPlaceholder, endPlaceholder)
	}
	placeholderStart := subMatchIndexes[2] + 1
	placeHolderEnd := subMatchIndexes[3] - 1

	buf := bytes.NewBuffer(nil)
	buf.Write(text[:placeholderStart])
	buf.Write(replaceContent)
	buf.Write(text[placeHolderEnd:])
	return buf.Bytes(), nil
}

// newRoleFromClusterRoleName generates user-authz role name from ClusterRole name
func newRoleFromClusterRoleName(name string) string {
	return strcase.ToCamel(strings.TrimPrefix(name, "user-authz:"))
}

// sliceToString sorts slice of strings and return joined by comma string
func sliceToString(s []string) string {
	return strings.Join(sortStrings(s), ",")
}

// sortStrings return new sorted slice
func sortStrings(s []string) []string {
	rs := make([]string, len(s))
	copy(rs, s)
	sort.Strings(rs)
	return rs
}
