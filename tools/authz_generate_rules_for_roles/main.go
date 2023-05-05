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
It inserts data between lines "<!-- start placeholder -->" and "<!-- end placeholder -->".
It useses rendered template from /deckhouse/modules/140-user-authz/templates/cluster-roles.yaml
Steps to use:
  - cd tools && go generate
  - make lint-markdown-fix
  - check diff for ./modules/140-user-authz/docs/README.md and ./modules/140-user-authz/docs/README_RU.md files
*/

package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v3"
)

var (
	readVerbs      = []string{"get", "list", "watch"}
	writeVerbs     = []string{"create", "delete", "deletecollection", "patch", "update"}
	readWriteVerbs = append(readVerbs, writeVerbs...)

	readVerbsString      = sliceToString(readVerbs)
	writeVerbsString     = sliceToString(writeVerbs)
	readWriteVerbsString = sliceToString(readWriteVerbs)

	readAlias      = "read"
	writeAlias     = "write"
	readWriteAlias = "read-write"

	clusterRoleModulePrefix = "user-authz:"

	userString           = "user"
	privilegedUserString = "privileged-user"
	editorString         = "editor"
	adminString          = "admin"
	clusterEditorString  = "cluster-editor"
	clusterAdminString   = "cluster-admin"

	userClusterRole           = clusterRoleModulePrefix + userString
	privilegedUserClusterRole = clusterRoleModulePrefix + privilegedUserString
	editorClusterRole         = clusterRoleModulePrefix + editorString
	adminClusterRole          = clusterRoleModulePrefix + adminString
	clusterEditorClusterRole  = clusterRoleModulePrefix + clusterEditorString
	clusterAdminClusterRole   = clusterRoleModulePrefix + clusterAdminString

	orderedRoleNames = []string{
		userClusterRole, privilegedUserClusterRole,
		editorClusterRole, adminClusterRole,
		clusterEditorClusterRole, clusterAdminClusterRole,
	}

	neededClusterRoleExcludes = map[string][]string{
		userClusterRole:           {},
		privilegedUserClusterRole: {userClusterRole},
		editorClusterRole:         {userClusterRole, privilegedUserClusterRole},
		adminClusterRole:          {userClusterRole, privilegedUserClusterRole, editorClusterRole},
		clusterEditorClusterRole:  {userClusterRole, privilegedUserClusterRole, editorClusterRole},
		clusterAdminClusterRole:   {userClusterRole, privilegedUserClusterRole, editorClusterRole, adminClusterRole, clusterEditorClusterRole},
	}
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}

	if err := os.Chdir("../"); err != nil {
		log.Fatalln(err)
	}

	renderContents, err := renderTemplates()
	if err != nil {
		log.Fatalln(err)
	}

	if err := os.Chdir(cwd); err != nil {
		log.Fatalln(err)
	}

	clusterRolesMap := getClusterRoles(renderContents)
	if err != nil {
		log.Fatalln(err)
	}

	crVerbResourceMap := make(map[string]map[string][]string)
	for name, rules := range clusterRolesMap {
		if _, f := neededClusterRoleExcludes[name]; !f {
			continue
		}
		crVerbResourceMap[name] = processClusterRoleRules(rules)
	}

	builder := &strings.Builder{}
	builder.WriteString("* read - `get`, `list`, `watch`\n")
	builder.WriteString("* read-write - `get`, `list`, `watch`, `create`, `delete`, `deletecollection`, `patch`, `update`\n")
	builder.WriteString("* write - `create`, `delete`, `deletecollection`, `patch`, `update`\n")
	builder.WriteString("\n```yaml\n")
	for _, name := range orderedRoleNames {
		if err := prepareContents(name, crVerbResourceMap, builder); err != nil {
			log.Fatalln(err)
		}
	}
	builder.WriteString("```")

	for _, fileName := range os.Args[1:] {
		if err := updateReame(fileName, builder.String()); err != nil {
			log.Fatalln(err)
		}
	}
}

func renderTemplates() ([]byte, error) {
	tempFile, err := os.CreateTemp("/tmp", "render-template-*.yaml")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	makeEnvs := []string{"USER_AUTHZ_RENDER_ROLES=yes", fmt.Sprintf("USER_AUTHZ_RENDER_FILE=%s", tempFile.Name()), "CGO_ENABLED=0"}
	if err := runMake("tests-modules", makeEnvs...); err != nil {
		return nil, err
	}

	renderContents, err := io.ReadAll(tempFile)
	if err != nil {
		return nil, err
	}
	return renderContents, nil
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

func getClusterRoles(contents []byte) map[string][]rule {
	dec := yaml.NewDecoder(bytes.NewReader(contents))
	roleMap := make(map[string][]rule)
	for {
		var role clusterRole
		if err := dec.Decode(&role); err != nil || len(role.Rules) < 1 {
			if errors.Is(err, io.EOF) {
				return roleMap
			}
			continue
		}
		roleMap[role.Metadata.Name] = role.Rules
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
	case readVerbsString:
		alias = readAlias
	case writeVerbsString:
		alias = writeAlias
	case readWriteVerbsString:
		alias = readWriteAlias
	}
	return alias, grSlice
}

func prepareContents(name string, crVerbResourceMap map[string]map[string][]string, builder *strings.Builder) error {
	newHeader := fmt.Sprintf("Role `%s`", camelCase(name))
	if excls := neededClusterRoleExcludes[name]; len(excls) > 0 {
		newExcls := make([]string, 0, len(excls))
		for _, v := range excls {
			newExcls = append(newExcls, camelCase(v))
		}
		newHeader = fmt.Sprintf("%s (and all rules from `%s`)", newHeader, strings.Join(newExcls, "`, `"))
	}

	m, err := yaml.Marshal(map[string]interface{}{newHeader: clusterRoleGenerateExcludes(name, crVerbResourceMap)})
	if err != nil {
		return err
	}
	builder.Write(m)
	builder.WriteString("\n")
	return nil
}

func clusterRoleGenerateExcludes(name string, rulesMap map[string]map[string][]string) map[string][]string {
	excludeNames := neededClusterRoleExcludes[name]
	excludesMap := make(map[string][]string)
	for _, name := range excludeNames {
		for verb, resources := range rulesMap[name] {
			excludesMap[verb] = append(excludesMap[verb], resources...)
		}
	}

	resultMap := make(map[string][]string, len(rulesMap))
	for verb, resources := range rulesMap[name] {
		resultResources := make([]string, 0)
		for _, resource := range resources {
			if isInSlice(resource, excludesMap[verb]) {
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

func updateReame(filePath string, contents string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	skip := false
	newFileContents := make([]string, 0)
	for scanner.Scan() {
		txt := scanner.Text()
		if strings.Contains(txt, "end placeholder") {
			skip = false
		}
		if skip {
			continue
		}
		newFileContents = append(newFileContents, txt)

		if strings.Contains(txt, "start placeholder") {
			skip = true
			newFileContents = append(newFileContents, contents)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if err := os.WriteFile(filePath, []byte(strings.Join(newFileContents, "\n")+"\n"), 0644); err != nil {
		return err
	}
	return nil
}

func isInSlice(str string, slice []string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
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

func camelCase(s string) string {
	return strcase.ToCamel(strings.TrimPrefix(s, clusterRoleModulePrefix))
}

func runMake(command string, envs ...string) error {
	cmd := exec.Command("make", command, "FOCUS=user-authz")
	cmd.Env = append(os.Environ(), envs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println(string(out))
		return err
	}
	return nil
}
