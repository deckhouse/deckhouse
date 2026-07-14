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
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/testing/library/helm"

	"tools/helm_generate/helper"
)

const (
	accessLevelAnnotation = "user-authz.deckhouse.io/access-level"
	moduleAuthzRolesFile  = "user-authz-cluster-roles.yaml"

	userRole           = "User"
	privilegedUserRole = "PrivilegedUser"
	editorRole         = "Editor"
	adminRole          = "Admin"
	clusterEditorRole  = "ClusterEditor"
	clusterAdminRole   = "ClusterAdmin"
)

var moduleRoots = []string{
	"modules",
	"ee/modules",
	"ee/be/modules",
	"ee/fe/modules",
	"ee/se/modules",
	"ee/se-plus/modules",
}

var (
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
		clusterAdminRole: {
			userRole,
			privilegedUserRole,
			editorRole,
			adminRole,
			clusterEditorRole,
		},
	}
)

// readmeTemplateData is data for readme template.
// The placeholder generates only the dynamic part (aliases + per-role rule
// blocks); static intro text lives in each README outside the placeholder so
// the EN and RU files contain only their own language (dmt-lint forbids
// cyrillic in README.md).
type readmeTemplateData struct {
	Roles   []role
	Aliases []alias
}

// role is representation of ClusterRole verbs for readme template
type role struct {
	Name            string
	Rules           roleRules
	AdditionalRoles []string
}

type roleRules map[string][]string

// alias is representation of commonly used verbs group for readme template
type alias struct {
	Name  string
	Verbs []string
	// verbsJoined represents "Verbs" joined with comma
	// (refer to newAlias)
	verbsJoined string
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

func run() error {
	deckhouseRoot, err := helper.DeckhouseRoot()
	if err != nil {
		return fmt.Errorf("get deckhouse root: %w", err)
	}

	// "github.com/deckhouse/deckhouse/testing/library" InitValues() can be used to seed render values.
	renderContents, err := renderHelmTemplates(
		deckhouseRoot,
		filepath.Join(deckhouseRoot, "modules/140-user-authz"),
		`{"userAuthz":{"internal":{}},"global":{}}`,
	)
	if err != nil {
		return fmt.Errorf("render user-authz templates: %w", err)
	}

	clusterRolesMap, err := getClusterRoles(renderContents, "user-authz/templates/cluster-roles.yaml")
	if err != nil {
		return fmt.Errorf("get user-authz cluster roles: %w", err)
	}

	// map[<role name>]map[<resource target>]<verbs>
	baseRoleRules := make(map[string]roleRuleSet, len(neededClusterRoleExcludes))
	for name, rules := range clusterRolesMap {
		if _, f := neededClusterRoleExcludes[name]; !f {
			continue
		}
		baseRoleRules[name] = processClusterRoleRules(rules)
	}

	if err := mergeModuleAuthzRoles(deckhouseRoot, baseRoleRules); err != nil {
		return fmt.Errorf("collect module authz roles: %w", err)
	}

	templateData := &readmeTemplateData{
		Aliases: aliases,
		Roles:   make([]role, 0, len(orderedRoleNames)),
	}
	for _, roleName := range orderedRoleNames {
		templateData.Roles = append(
			templateData.Roles,
			prepareClusterRoleForTemplate(
				roleName,
				neededClusterRoleExcludes[roleName],
				baseRoleRules,
			),
		)
	}

	readmeContent, err := renderTemplate(
		filepath.Join(deckhouseRoot, "tools/helm_generate/runners/authz_generate_rules_for_roles/readme-placeholder.tpl"),
		templateData,
	)
	if err != nil {
		return fmt.Errorf("render readme placeholder: %w", err)
	}

	readmeFiles := []string{
		filepath.Join(deckhouseRoot, "modules/140-user-authz/docs/README.md"),
		filepath.Join(deckhouseRoot, "modules/140-user-authz/docs/README_RU.md"),
	}
	for _, fileName := range readmeFiles {
		if err := updateReadme(fileName, readmeContent); err != nil {
			return fmt.Errorf("update %s: %w", fileName, err)
		}
	}
	return nil
}

// renderHelmTemplates renders helm template for chart directory "dir" with values "values"
func renderHelmTemplates(deckhouseRoot, dir, values string) (map[string]string, error) {
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
		if err := os.Remove(chartHelmLibPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "warning: restore chart helm_lib symlink: remove %s: %v\n", chartHelmLibPath, err)
			return
		}
		// Restore the in-container symlink so the chart stays valid for Docker-based renders.
		restoreTarget := filepath.Join("/deckhouse", helmLibPath)
		if err := os.Symlink(restoreTarget, chartHelmLibPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: restore chart helm_lib symlink: %v\n", err)
		}
	}()

	r := helm.Renderer{}
	return r.RenderChartFromDir(dir, values)
}

type clusterRole struct {
	Metadata struct {
		Name        string            `yaml:"name"`
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
	Rules []rule `yaml:"rules"`
}

type rule struct {
	Verbs           []string `yaml:"verbs"`
	APIGroups       []string `yaml:"apiGroups"`
	Resources       []string `yaml:"resources"`
	ResourceNames   []string `yaml:"resourceNames,omitempty"`
	NonResourceURLs []string `yaml:"nonResourceURLs,omitempty"`
}

// getClusterRoles retrieves ClusterRole template file "fileName" from helm templates map "templates"
// and converts ClusterRoles verbs and resources to map.
func getClusterRoles(
	templates map[string]string,
	fileName string,
) (map[string][]rule, error) {
	body, ok := templates[fileName]
	if !ok {
		return nil, fmt.Errorf("rendered template %q not found", fileName)
	}

	dec := yaml.NewDecoder(strings.NewReader(body))
	roleMap := make(map[string][]rule)
	for {
		var r clusterRole
		err := dec.Decode(&r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return roleMap, nil
			}
			return nil, fmt.Errorf("yaml decode %s: %w", fileName, err)
		}
		if r.Metadata.Name == "" || len(r.Rules) < 1 {
			continue
		}
		roleName := newRoleFromClusterRoleName(r.Metadata.Name)
		if _, dup := roleMap[roleName]; dup {
			return nil, fmt.Errorf("duplicate cluster role %q in %s (from ClusterRole %q)", roleName, fileName, r.Metadata.Name)
		}
		roleMap[roleName] = r.Rules
	}
}

type permissionTarget struct {
	APIGroup       string
	Resource       string
	ResourceNames  []string
	NonResourceURL string
}

type targetRule struct {
	target permissionTarget
	verbs  map[string]struct{}
}

type roleRuleSet map[string]*targetRule

// key returns a stable identifier for the target. It is called once per rule
// insertion, so we avoid the intermediate []string + strings.Join allocations
// by writing directly into a pre-sized strings.Builder (single allocation).
func (t permissionTarget) key() string {
	n := len(t.APIGroup) + len(t.Resource) + len(t.NonResourceURL) + 3 // 3 outer "\x01" separators
	for _, name := range t.ResourceNames {
		n += len(name)
	}
	if len(t.ResourceNames) > 1 {
		n += len(t.ResourceNames) - 1 // inner "\x00" separators
	}

	var b strings.Builder
	b.Grow(n)
	b.WriteString(t.APIGroup)
	b.WriteByte('\x01')
	b.WriteString(t.Resource)
	b.WriteByte('\x01')
	for i, name := range t.ResourceNames {
		if i > 0 {
			b.WriteByte('\x00')
		}
		b.WriteString(name)
	}
	b.WriteByte('\x01')
	b.WriteString(t.NonResourceURL)
	return b.String()
}

func (t permissionTarget) displayName() string {
	if t.NonResourceURL != "" {
		return "nonResourceURLs/" + t.NonResourceURL
	}

	name := t.Resource
	if t.APIGroup != "" {
		name = t.APIGroup + "/" + t.Resource
	}
	if len(t.ResourceNames) > 0 {
		resourceNames := strings.Join(sortStrings(t.ResourceNames), ", ")
		name = fmt.Sprintf("%s (resourceNames: %s)", name, resourceNames)
	}
	return name
}

func (rs roleRuleSet) add(target permissionTarget, verbs []string) {
	rule := rs.ensure(target, len(verbs))
	for _, verb := range verbs {
		rule.verbs[verb] = struct{}{}
	}
}

func (rs roleRuleSet) ensure(target permissionTarget, verbCount int) *targetRule {
	target.ResourceNames = sortStrings(target.ResourceNames)
	key := target.key()
	if rs[key] == nil {
		rs[key] = &targetRule{
			target: target,
			verbs:  make(map[string]struct{}, verbCount),
		}
	}
	return rs[key]
}

func mergeRoleRuleSet(dst, src roleRuleSet) {
	for _, tr := range src {
		dstRule := dst.ensure(tr.target, len(tr.verbs))
		for verb := range tr.verbs {
			dstRule.verbs[verb] = struct{}{}
		}
	}
}

// processClusterRoleRules generates a map of resource targets with verb sets.
func processClusterRoleRules(rules []rule) roleRuleSet {
	rulesMap := make(roleRuleSet)
	for _, r := range rules {
		processRule(r, rulesMap)
	}
	return rulesMap
}

// processRule retrieves resource targets and verb sets from rule.
func processRule(r rule, rulesMap roleRuleSet) {
	for _, url := range r.NonResourceURLs {
		rulesMap.add(permissionTarget{NonResourceURL: url}, r.Verbs)
	}

	apiGroups := r.APIGroups
	if len(apiGroups) == 0 && len(r.Resources) > 0 {
		apiGroups = []string{""}
	}
	for _, apiGroup := range apiGroups {
		for _, resource := range r.Resources {
			rulesMap.add(permissionTarget{
				APIGroup:      apiGroup,
				Resource:      resource,
				ResourceNames: r.ResourceNames,
			}, r.Verbs)
		}
	}
}

// verbsKey returns the canonical key for a verb set: either a registered alias
// name or a sorted, comma-joined list. It sorts the input slice in place; the
// caller MUST NOT rely on the original ordering after the call.
func verbsKey(verbs []string) string {
	slices.Sort(verbs)
	verbsJoined := strings.Join(verbs, ",")
	if al, f := isAliased(verbsJoined); f {
		return al
	}
	return verbsJoined
}

// prepareClusterRoleForTemplate generates template data for ClusterRole.
func prepareClusterRoleForTemplate(
	roleName string,
	excls []string,
	baseRoleRules map[string]roleRuleSet,
) role {
	templateRole := role{
		Name: roleName,
	}

	excludesMap := make(roleRuleSet)
	if len(excls) > 0 {
		templateRole.AdditionalRoles = excls
		excludesMap = clusterRoleGenerateExcludes(excls, baseRoleRules)
	}

	templateRole.Rules = clusterRoleApplyExcludes(baseRoleRules[roleName], excludesMap)
	return templateRole
}

// clusterRoleGenerateExcludes generates target/verb exclusions from included roles.
func clusterRoleGenerateExcludes(
	excludeRoleNames []string,
	rulesMap map[string]roleRuleSet,
) roleRuleSet {
	excludesMap := make(roleRuleSet)
	if len(excludeRoleNames) < 1 {
		return excludesMap
	}

	for _, name := range excludeRoleNames {
		mergeRoleRuleSet(excludesMap, rulesMap[name])
	}
	return excludesMap
}

// clusterRoleApplyExcludes removes verbs from "roleRules" already covered by "excludesMap".
func clusterRoleApplyExcludes(
	sourceRules roleRuleSet,
	excludesMap roleRuleSet,
) roleRules {
	resultMap := make(roleRules, len(sourceRules))
	for key, tr := range sourceRules {
		var excludedVerbs map[string]struct{}
		if excludedRule := excludesMap[key]; excludedRule != nil {
			excludedVerbs = excludedRule.verbs
		}

		verbs := make([]string, 0, len(tr.verbs))
		if _, wildcardExcluded := excludedVerbs["*"]; !wildcardExcluded {
			for verb := range tr.verbs {
				if _, f := excludedVerbs[verb]; f {
					continue
				}
				verbs = append(verbs, verb)
			}
		}

		if len(verbs) < 1 {
			continue
		}
		verbKey := verbsKey(verbs)
		resultMap[verbKey] = append(resultMap[verbKey], tr.target.displayName())
	}
	for verb, resources := range resultMap {
		resultMap[verb] = dedupSorted(resources)
	}
	return resultMap
}

// mergeModuleAuthzRoles adds default module roles annotated with access-level.
// Hooks bind each annotated ClusterRole to its own level and every senior level,
// so the generated README must apply the same cumulative module merge.
func mergeModuleAuthzRoles(deckhouseRoot string, roleRules map[string]roleRuleSet) error {
	modules, err := findModulesWithAuthzRoles(deckhouseRoot)
	if err != nil {
		return err
	}

	for _, m := range modules {
		roles, err := renderModuleAuthzRoles(m.path, m.chartName)
		if err != nil {
			return fmt.Errorf("render module %q authz roles: %w", m.chartName, err)
		}
		for _, cr := range roles {
			level := cr.Metadata.Annotations[accessLevelAnnotation]
			levelIndex := slices.Index(orderedRoleNames, level)
			if levelIndex < 0 {
				continue
			}
			clusterRoleRules := processClusterRoleRules(cr.Rules)
			for _, roleName := range orderedRoleNames[levelIndex:] {
				if roleRules[roleName] == nil {
					roleRules[roleName] = make(roleRuleSet)
				}
				mergeRoleRuleSet(roleRules[roleName], clusterRoleRules)
			}
		}
	}
	return nil
}

type moduleEntry struct {
	path      string
	chartName string
}

func findModulesWithAuthzRoles(deckhouseRoot string) ([]moduleEntry, error) {
	var result []moduleEntry
	for _, root := range moduleRoots {
		rootPath := filepath.Join(deckhouseRoot, root)
		entries, err := os.ReadDir(rootPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read modules root %s: %w", rootPath, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			modPath := filepath.Join(rootPath, e.Name())
			moduleAuthzRolesPath := filepath.Join(modPath, "templates", moduleAuthzRolesFile)
			if _, err := os.Stat(moduleAuthzRolesPath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return nil, fmt.Errorf("stat module authz roles %s: %w", moduleAuthzRolesPath, err)
			}
			chartName, err := readChartName(filepath.Join(modPath, "Chart.yaml"))
			if err != nil {
				return nil, fmt.Errorf("read module chart name %s: %w", modPath, err)
			}
			result = append(result, moduleEntry{path: modPath, chartName: chartName})
		}
	}
	slices.SortFunc(result, func(a, b moduleEntry) int {
		return cmp.Compare(a.path, b.path)
	})
	return result, nil
}

func readChartName(chartYaml string) (string, error) {
	data, err := os.ReadFile(chartYaml)
	if err != nil {
		return "", fmt.Errorf("read chart file: %w", err)
	}
	var c struct {
		Name string `yaml:"name"`
	}
	if err := yaml.Unmarshal(data, &c); err != nil {
		return "", fmt.Errorf("unmarshal chart file: %w", err)
	}
	if c.Name == "" {
		return "", fmt.Errorf("empty chart name in %s", chartYaml)
	}
	return c.Name, nil
}

func renderModuleAuthzRoles(modulePath, chartName string) ([]clusterRole, error) {
	rd, err := helper.NewRenderDir(chartName)
	if err != nil {
		return nil, fmt.Errorf("render dir: %w", err)
	}
	defer rd.Remove()

	tplPath := filepath.Join(modulePath, "templates", moduleAuthzRolesFile)
	if err := rd.AddTemplate(moduleAuthzRolesFile, tplPath); err != nil {
		return nil, fmt.Errorf("add template: %w", err)
	}
	rd.AddHelper(filepath.Join(modulePath, "templates"))

	r := helm.Renderer{}
	contents, err := r.RenderChartFromDir(rd.Path(), "{}")
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}

	for name, body := range contents {
		if strings.Contains(name, "templates/"+moduleAuthzRolesFile) {
			roles, err := decodeClusterRoles(body)
			if err != nil {
				return nil, fmt.Errorf("decode rendered roles: %w", err)
			}
			return roles, nil
		}
	}
	return nil, nil
}

func decodeClusterRoles(yamlText string) ([]clusterRole, error) {
	var roles []clusterRole
	dec := yaml.NewDecoder(strings.NewReader(yamlText))
	for {
		var cr clusterRole
		if err := dec.Decode(&cr); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if cr.Metadata.Name == "" || len(cr.Rules) == 0 {
			continue
		}
		roles = append(roles, cr)
	}
	return roles, nil
}

func dedupSorted(s []string) []string {
	sorted := sortStrings(s)
	return slices.Compact(sorted)
}

// updateReadme opens "fileName" file and replaces it's contents
// between "<!-- start user-authz roles placeholder -->" and
// "<!-- end user-authz roles placeholder -->" with "content"
func updateReadme(fileName string, content []byte) error {
	const (
		startPlaceholder = "<!-- start user-authz roles placeholder -->"
		endPlaceholder   = "<!-- end user-authz roles placeholder -->"
	)

	fileText, err := os.ReadFile(fileName)
	if err != nil {
		return err
	}

	newFileContents, err := replacePlaceholder(fileText, content, startPlaceholder, endPlaceholder)
	if err != nil {
		return err
	}

	if err := os.WriteFile(fileName, newFileContents, 0o644); err != nil {
		return err
	}
	return nil
}

// renderTemplate renders template from "templateFile" file with readme values.
func renderTemplate(templateFile string, templateData *readmeTemplateData) ([]byte, error) {
	templateFuncMap := sprig.TxtFuncMap()

	templateFuncMap["toYaml"] = func(v any) (string, error) {
		output, err := yaml.Marshal(v)
		return string(output), err
	}
	tpl, err := template.New(filepath.Base(templateFile)).
		Funcs(templateFuncMap).
		ParseFiles(templateFile)
	if err != nil {
		return nil, err
	}

	var res bytes.Buffer
	if err := tpl.Execute(&res, templateData); err != nil {
		return nil, err
	}

	return bytes.TrimSpace(res.Bytes()), nil
}

// replacePlaceholder replaces contents in "text" between "startPlaceholder" and "endPlaceholder" with "replaceContent"
func replacePlaceholder(text, replaceContent []byte, startPlaceholder, endPlaceholder string) ([]byte, error) {
	start := bytes.Index(text, []byte(startPlaceholder))
	if start < 0 {
		return nil, fmt.Errorf("didn't find submatch inside placeholder `%s` and `%s`", startPlaceholder, endPlaceholder)
	}

	replaceStart := start + len(startPlaceholder)
	if replaceStart < len(text) && text[replaceStart] == '\n' {
		replaceStart++
	}

	replaceEnd := bytes.Index(text[replaceStart:], []byte(endPlaceholder))
	if replaceEnd < 0 {
		return nil, fmt.Errorf("didn't find submatch inside placeholder `%s` and `%s`", startPlaceholder, endPlaceholder)
	}
	replaceEnd += replaceStart
	if replaceEnd > replaceStart && text[replaceEnd-1] == '\n' {
		replaceEnd--
	}

	result := make([]byte, 0, len(text)-replaceEnd+replaceStart+len(replaceContent))
	result = append(result, text[:replaceStart]...)
	result = append(result, replaceContent...)
	result = append(result, text[replaceEnd:]...)
	return result, nil
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
	rs := slices.Clone(s)
	slices.Sort(rs)
	return rs
}
