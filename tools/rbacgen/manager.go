/*
Copyright 2024 Flant JSC

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

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	apimachineryv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const allResources = "all"

var scopeTemplate = "rbac.deckhouse.io/aggregate-to-%s-as"

type moduleGenerator struct {
	module             string
	namespace          string
	path               string
	crds               []string
	scopes             []string
	allowedResources   []resource
	forbiddenResources []string
	buffer             []byte
}

type settings struct {
	Module             string     `yaml:"module"`
	Namespace          string     `json:"namespace"`
	Scopes             []string   `yaml:"scopes"`
	CRDs               []string   `yaml:"crds"`
	AllowedResources   []resource `yaml:"allowedResources"`
	ForbiddenResources []string   `yaml:"forbiddenResources"`
	path               string
}

type resource struct {
	Group     string   `yaml:"group"`
	Resources []string `yaml:"resources"`
}

func newModuleGenerator(settings settings) (*moduleGenerator, error) {
	var crds []string
	for _, dir := range settings.CRDs {
		crdsInDir, err := filepath.Glob(dir)
		if err != nil {
			return nil, err
		}
		crds = append(crds, crdsInDir...)
	}
	return &moduleGenerator{
		module:             settings.Module,
		namespace:          settings.Namespace,
		path:               settings.path,
		crds:               crds,
		scopes:             settings.Scopes,
		allowedResources:   settings.AllowedResources,
		forbiddenResources: settings.ForbiddenResources,
		buffer:             make([]byte, 1*1024*1024),
	}, nil
}

func (m *moduleGenerator) generate(ctx context.Context) (moduleDoc, error) {
	manageResources, useResources, err := m.parseCRDs(ctx)
	if err != nil {
		return moduleDoc{}, err
	}
	manageRoles, userRoles := m.buildRoles(manageResources, useResources)
	return m.buildDoc(manageRoles, userRoles), m.writeRoles(manageRoles, userRoles)
}

func (m *moduleGenerator) buildRoles(manageResources, useResources map[string][]string) ([]*rbacv1.ClusterRole, []*rbacv1.ClusterRole) {
	var useViewRules, useEditRules, manageViewRules, manageEditRules []rbacv1.PolicyRule
	if manageResources != nil && len(manageResources) != 0 {
		for group, resources := range manageResources {
			manageViewRules = append(manageViewRules, rbacv1.PolicyRule{
				APIGroups: []string{group},
				Resources: resources,
				Verbs:     []string{"get", "list", "watch"},
			})
			manageEditRules = append(manageEditRules, rbacv1.PolicyRule{
				APIGroups: []string{group},
				Resources: resources,
				Verbs:     []string{"create", "update", "patch", "delete", "deletecollection"},
			})
		}
	}
	if useResources != nil && len(useResources) != 0 {
		for group, resources := range useResources {
			useViewRules = append(useViewRules, rbacv1.PolicyRule{
				APIGroups: []string{group},
				Resources: resources,
				Verbs:     []string{"get", "list", "watch"},
			})
			useEditRules = append(useEditRules, rbacv1.PolicyRule{
				APIGroups: []string{group},
				Resources: resources,
				Verbs:     []string{"create", "update", "patch", "delete", "deletecollection"},
			})
		}
	}
	//deckhouse can manage all module configs
	if m.module != "deckhouse" {
		manageViewRules = append(manageViewRules, rbacv1.PolicyRule{
			APIGroups:     []string{"deckhouse.io"},
			Resources:     []string{"moduleconfigs"},
			ResourceNames: []string{m.module},
			Verbs:         []string{"get", "list", "watch"},
		})
		manageEditRules = append(manageEditRules, rbacv1.PolicyRule{
			APIGroups:     []string{"deckhouse.io"},
			Resources:     []string{"moduleconfigs"},
			ResourceNames: []string{m.module},
			Verbs:         []string{"create", "update", "patch", "delete"},
		})
	}
	var manageRoles = []*rbacv1.ClusterRole{
		m.buildRole("viewer", "manage", "view", manageViewRules),
		m.buildRole("manager", "manage", "edit", manageEditRules),
	}
	var useRoles []*rbacv1.ClusterRole
	if useViewRules != nil {
		useRoles = append(useRoles, m.buildRole("viewer", "use", "view", useViewRules))
	}
	if useEditRules != nil {
		useRoles = append(useRoles, m.buildRole("manager", "use", "edit", useEditRules))
	}
	return manageRoles, useRoles
}
func (m *moduleGenerator) buildRole(rbacRole, rbacKind, rbacVerb string, rules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	var role *rbacv1.ClusterRole
	if rbacKind == "use" {
		role = &rbacv1.ClusterRole{
			TypeMeta: apimachineryv1.TypeMeta{
				APIVersion: rbacv1.SchemeGroupVersion.String(),
				Kind:       "ClusterRole",
			},
			ObjectMeta: apimachineryv1.ObjectMeta{
				Name: fmt.Sprintf("d8:%s:capability:module:%s:%s", rbacKind, m.module, rbacVerb),
				Labels: map[string]string{
					"heritage":                            "deckhouse",
					"module":                              m.module,
					"rbac.deckhouse.io/kind":              rbacKind,
					"rbac.deckhouse.io/aggregate-to-role": rbacRole,
				},
			},
			Rules: rules,
		}
	}
	if rbacKind == "manage" {
		role = &rbacv1.ClusterRole{
			TypeMeta: apimachineryv1.TypeMeta{
				APIVersion: rbacv1.SchemeGroupVersion.String(),
				Kind:       "ClusterRole",
			},
			ObjectMeta: apimachineryv1.ObjectMeta{
				Name: fmt.Sprintf("d8:%s:capability:module:%s:%s", rbacKind, m.module, rbacVerb),
				Labels: map[string]string{
					"heritage":                "deckhouse",
					"module":                  m.module,
					"rbac.deckhouse.io/kind":  rbacKind,
					"rbac.deckhouse.io/level": "module",
				},
			},
			Rules: rules,
		}
		for _, scope := range m.scopes {
			role.ObjectMeta.Labels[fmt.Sprintf(scopeTemplate, scope)] = rbacRole
		}
		if m.namespace != "none" {
			if m.namespace == "" {
				role.ObjectMeta.Labels["rbac.deckhouse.io/namespace"] = fmt.Sprintf("d8-%s", m.module)
			} else {
				role.ObjectMeta.Labels["rbac.deckhouse.io/namespace"] = m.namespace
			}
		}
	}
	return role
}

func (m *moduleGenerator) writeRoles(manageRoles, useRoles []*rbacv1.ClusterRole) error {
	for _, role := range manageRoles {
		var name = "edit"
		if strings.HasSuffix(role.Name, "view") {
			name = "view"
		}
		if err := m.ensurePath("templates/rbacv2/manage"); err != nil {
			return err
		}
		marshaled, err := yaml.Marshal(role)
		managePath := filepath.Join(m.path, "templates", "rbacv2", "manage", fmt.Sprintf("%s.yaml", name))
		if err = os.WriteFile(managePath, marshaled, 0644); err != nil {
			return err
		}
	}
	for _, role := range useRoles {
		var name = "edit"
		if strings.HasSuffix(role.Name, "view") {
			name = "view"
		}
		if err := m.ensurePath("templates/rbacv2/use"); err != nil {
			return err
		}
		marshaled, err := yaml.Marshal(role)
		if err != nil {
			return err
		}
		usePath := filepath.Join(m.path, "templates", "rbacv2", "use", fmt.Sprintf("%s.yaml", name))
		if err = os.WriteFile(usePath, marshaled, 0644); err != nil {
			return err
		}
	}
	return nil
}

func (m *moduleGenerator) ensurePath(path string) error {
	return os.MkdirAll(filepath.Join(m.path, path), 0755)
}

func (m *moduleGenerator) allowResource(group, resource string) bool {
	for _, r := range m.allowedResources {
		if r.Group == group && (slices.Contains(r.Resources, resource) || r.Resources[0] == allResources) {
			return true
		}
	}
	return false
}
