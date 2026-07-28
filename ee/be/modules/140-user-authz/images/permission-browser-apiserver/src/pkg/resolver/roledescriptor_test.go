/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
)

func TestDescribeRole(t *testing.T) {
	tests := []struct {
		name     string
		meta     *metav1.ObjectMeta
		roleName string
		expected v1alpha1.RoleDescriptor
	}{
		{
			name: "labels are authoritative",
			meta: &metav1.ObjectMeta{Labels: map[string]string{
				labelRoleKind:  "role",
				labelRoleScope: "subsystem",
				labelSubsystem: "networking",
			}},
			roleName: "d8:subsystem:networking:manager",
			expected: v1alpha1.RoleDescriptor{
				Kind: "role", Scope: "subsystem", Subsystem: "networking", Level: "manager",
			},
		},
		{
			name:     "scope and level fall back to the name",
			meta:     &metav1.ObjectMeta{},
			roleName: "d8:namespace:admin",
			expected: v1alpha1.RoleDescriptor{Scope: "namespace", Level: "admin"},
		},
		{
			name:     "system role",
			meta:     nil,
			roleName: "d8:system:superadmin",
			expected: v1alpha1.RoleDescriptor{Scope: "system", Level: "superadmin"},
		},
		{
			name:     "project role",
			meta:     nil,
			roleName: "d8:project:viewer",
			expected: v1alpha1.RoleDescriptor{Scope: "project", Level: "viewer"},
		},
		{
			name:     "custom role keeps its scope",
			meta:     nil,
			roleName: "d8:custom:subsystem:mycustom:manager",
			expected: v1alpha1.RoleDescriptor{
				Kind: "custom-role", Scope: "subsystem", Subsystem: "mycustom", Level: "manager",
			},
		},
		{
			name:     "capability is recognised by name",
			meta:     nil,
			roleName: "d8:system-capability:deckhouse:view",
			expected: v1alpha1.RoleDescriptor{Kind: "capability"},
		},
		{
			name:     "deprecated manage alias",
			meta:     nil,
			roleName: "d8:manage:networking:manager",
			expected: v1alpha1.RoleDescriptor{Scope: "subsystem", Subsystem: "networking", Level: "manager"},
		},
		{
			name:     "deprecated manage:all alias maps to the system scope",
			meta:     nil,
			roleName: "d8:manage:all:viewer",
			expected: v1alpha1.RoleDescriptor{Scope: "system", Level: "viewer"},
		},
		{
			name:     "deprecated use alias maps to the namespace scope",
			meta:     nil,
			roleName: "d8:use:role:admin",
			expected: v1alpha1.RoleDescriptor{Scope: "namespace", Level: "admin"},
		},
		{
			name:     "roles outside the model stay undescribed",
			meta:     &metav1.ObjectMeta{},
			roleName: "cluster-admin",
			expected: v1alpha1.RoleDescriptor{},
		},
		{
			name: "deprecated label is reported",
			meta: &metav1.ObjectMeta{Labels: map[string]string{
				labelDeprecated: "true",
			}},
			roleName: "d8:use:role:viewer",
			expected: v1alpha1.RoleDescriptor{Scope: "namespace", Level: "viewer", Deprecated: true},
		},
		{
			name:     "an unknown level is not invented",
			meta:     nil,
			roleName: "d8:namespace:whatever",
			expected: v1alpha1.RoleDescriptor{Scope: "namespace"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, DescribeRole(tt.meta, tt.roleName))
		})
	}
}

func TestDescribeRole_Titles(t *testing.T) {
	meta := &metav1.ObjectMeta{Annotations: map[string]string{
		"en.meta.deckhouse.io/title":       "Namespace Administrator",
		"ru.meta.deckhouse.io/title":       "Администратор пространства имён",
		"en.meta.deckhouse.io/description": "Manages the namespace.",
	}}

	descriptor := DescribeRole(meta, "d8:namespace:admin")

	assert.Equal(t, "Namespace Administrator", descriptor.Titles["en"])
	assert.Equal(t, "Администратор пространства имён", descriptor.Titles["ru"])
	assert.Equal(t, "Manages the namespace.", descriptor.Descriptions["en"])
	assert.NotContains(t, descriptor.Descriptions, "ru")
}

func TestDescribeRole_CustomTitleOverridesShippedOne(t *testing.T) {
	meta := &metav1.ObjectMeta{Annotations: map[string]string{
		"en.meta.deckhouse.io/title":     "Namespace Administrator",
		"ru.meta.deckhouse.io/title":     "Администратор пространства имён",
		"custom.meta.deckhouse.io/title": "Team lead",
	}}

	descriptor := DescribeRole(meta, "d8:namespace:admin")

	// Renaming a built-in role is the one edit the role model allows, so the
	// operator's wording must win in every language.
	assert.Equal(t, "Team lead", descriptor.Titles["en"])
	assert.Equal(t, "Team lead", descriptor.Titles["ru"])
}

func TestIsSuperadminRole(t *testing.T) {
	tests := []struct {
		name       string
		descriptor v1alpha1.RoleDescriptor
		expected   bool
	}{
		{
			name:       "namespace superadmin",
			descriptor: v1alpha1.RoleDescriptor{Scope: "namespace", Level: "superadmin"},
			expected:   true,
		},
		{
			name:       "system superadmin",
			descriptor: v1alpha1.RoleDescriptor{Scope: "system", Level: "superadmin"},
			expected:   true,
		},
		{
			name:       "admin is not superadmin",
			descriptor: v1alpha1.RoleDescriptor{Scope: "namespace", Level: "admin"},
			expected:   false,
		},
		{
			name:       "superadmin of an unknown scope does not count",
			descriptor: v1alpha1.RoleDescriptor{Level: "superadmin"},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsSuperadminRole(tt.descriptor))
		})
	}
}

func TestIsClusterAdminRole(t *testing.T) {
	tests := []struct {
		name     string
		roleName string
		expected bool
	}{
		{
			name:     "kubernetes cluster-admin",
			roleName: "cluster-admin",
			expected: true,
		},
		{
			name:     "legacy SuperAdmin access level",
			roleName: "user-authz:super-admin",
			expected: true,
		},
		{
			name:     "legacy ClusterAdmin is one step below and does not count",
			roleName: "user-authz:cluster-admin",
			expected: false,
		},
		{
			name:     "an ordinary role does not count",
			roleName: "d8:namespace:admin",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsClusterAdminRole(tt.roleName))
		})
	}
}
