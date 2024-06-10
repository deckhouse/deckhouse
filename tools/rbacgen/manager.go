package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimachineryv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryYaml "k8s.io/apimachinery/pkg/util/yaml"
	"log"
	"os"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"slices"
	"strings"
)

const allResources = "all"

var scopeTemplate = "rbac.deckhouse.io/aggregate-to-%s"

type moduleGenerator struct {
	module             string
	path               string
	crds               []string
	scopes             []string
	allowedResources   []resource
	forbiddenResources []string
	buffer             []byte
}

type settings struct {
	Module             string     `yaml:"module"`
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
		path:               settings.path,
		crds:               crds,
		scopes:             settings.Scopes,
		allowedResources:   settings.AllowedResources,
		forbiddenResources: settings.ForbiddenResources,
		buffer:             make([]byte, 1*1024*1024),
	}, nil
}

func (m *moduleGenerator) generate(ctx context.Context) error {
	manageResources, useResources, err := m.parseCRDs(ctx)
	if err != nil {
		return err
	}
	return m.writeRoles(m.buildRoles(manageResources, useResources))
}

func (m *moduleGenerator) parseCRDs(ctx context.Context) (map[string][]string, map[string][]string, error) {
	var manageResources, useResources = make(map[string][]string), make(map[string][]string)
	for _, crd := range m.crds {
		if match := strings.HasPrefix(filepath.Base(crd), "doc-"); match {
			continue
		}
		parsed, err := m.processFile(ctx, crd)
		if err != nil {
			return nil, nil, err
		}
		if len(parsed) != 0 {
			for _, res := range parsed {
				if res.Spec.Scope == "Cluster" {
					manageResources[res.Spec.Group] = append(manageResources[res.Spec.Group], res.Spec.Names.Plural)
				} else {
					useResources[res.Spec.Group] = append(useResources[res.Spec.Group], res.Spec.Names.Plural)
				}
			}
		}
	}
	return manageResources, useResources, nil
}
func (m *moduleGenerator) processFile(ctx context.Context, path string) ([]*v1.CustomResourceDefinition, error) {
	fileReader, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fileReader.Close()
	var crds []*v1.CustomResourceDefinition
	reader := apimachineryYaml.NewDocumentDecoder(fileReader)
	for {
		n, err := reader.Read(m.buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		data := m.buffer[:n]
		if len(data) == 0 {
			// some empty yaml document, or empty string before separator
			continue
		}
		crd, err := m.parseCRD(ctx, bytes.NewReader(data), n)
		if err != nil {
			return nil, err
		}
		if crd != nil {
			crds = append(crds, crd)
		}
	}
	return crds, nil
}
func (m *moduleGenerator) parseCRD(_ context.Context, reader io.Reader, bufferSize int) (*v1.CustomResourceDefinition, error) {
	var crd *v1.CustomResourceDefinition
	if err := apimachineryYaml.NewYAMLOrJSONDecoder(reader, bufferSize).Decode(&crd); err != nil {
		return nil, err
	}
	// it could be a comment or some other peace of yaml file, skip it
	if crd == nil {
		return nil, nil
	}
	if crd.APIVersion != v1.SchemeGroupVersion.String() && crd.Kind != "CustomResourceDefinition" {
		return nil, fmt.Errorf("invalid CRD document apiversion/kind: '%s/%s'", crd.APIVersion, crd.Kind)
	}
	if crd.Spec.Group != "deckhouse.io" {
		if m.allowResource(crd.Spec.Group, crd.Spec.Names.Plural) {
			return crd, nil
		}
		return nil, nil
	}
	if slices.Contains(m.forbiddenResources, crd.Spec.Names.Plural) {
		return nil, nil
	}
	return crd, nil
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
	role := &rbacv1.ClusterRole{
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
	if rbacKind == "manage" {
		for _, scope := range m.scopes {
			role.ObjectMeta.Labels[fmt.Sprintf(scopeTemplate, scope)] = "true"
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
		if err != nil {
			return err
		}
		managePath := filepath.Join(m.path, "templates/rbacv2/manage", fmt.Sprintf("%s.yaml", name))
		log.Printf("render manage role %v in %s\n", role, managePath)
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
		usePath := filepath.Join(m.path, "templates/rbacv2/use", fmt.Sprintf("%s.yaml", name))
		log.Printf("render use role %v in %s\n", role, usePath)
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
