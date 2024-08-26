package main

import (
	"fmt"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type docs struct {
	Scopes  map[string]scopeDoc  `json:"scopes"`
	Modules map[string]moduleDoc `json:"modules"`
}
type scopeDoc struct {
	Modules       []string `json:"modules"`
	Namespaces    []string `json:"namespaces"`
	namespacesSet sets.String
}
type moduleDoc struct {
	Scopes       []string        `json:"scopes"`
	Capabilities capabilitiesDoc `json:"capabilities"`
	Namespace    string          `json:"namespace"`
}
type capabilitiesDoc struct {
	Manage []capabilityDoc `json:"manage"`
	Use    []capabilityDoc `json:"use"`
}
type capabilityDoc struct {
	Name  string              `json:"name"`
	Rules []rbacv1.PolicyRule `json:"rules"`
}

func (m *moduleGenerator) buildDoc(manageRoles, useRoles []*rbacv1.ClusterRole) moduleDoc {
	doc := moduleDoc{Scopes: m.scopes, Namespace: m.namespace}
	if doc.Namespace != "none" && m.namespace == "" {
		doc.Namespace = fmt.Sprintf("d8-%s", m.module)
	}
	for _, role := range manageRoles {
		doc.Capabilities.Manage = append(doc.Capabilities.Manage, capabilityDoc{
			Name:  role.Name,
			Rules: role.Rules,
		})
	}
	for _, role := range useRoles {
		doc.Capabilities.Use = append(doc.Capabilities.Use, capabilityDoc{
			Name:  role.Name,
			Rules: role.Rules,
		})
	}
	return doc
}
