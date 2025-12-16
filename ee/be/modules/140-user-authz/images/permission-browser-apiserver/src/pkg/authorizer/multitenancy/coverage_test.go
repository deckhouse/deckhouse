/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package multitenancy

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

// TestEngine_InitialConfigLoad tests initial configuration loading
func TestEngine_InitialConfigLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Configuration with a rule
	config := UserAuthzConfig{CRDs: []struct {
		Name string `json:"name"`
		Spec struct {
			AccessLevel                   string             `json:"accessLevel"`
			PortForwarding                bool               `json:"portForwarding"`
			AllowScale                    bool               `json:"allowScale"`
			AllowAccessToSystemNamespaces bool               `json:"allowAccessToSystemNamespaces"`
			LimitNamespaces               []string           `json:"limitNamespaces"`
			NamespaceSelector             *NamespaceSelector `json:"namespaceSelector"`
			AdditionalRoles               []struct {
				APIGroup string `json:"apiGroup"`
				Kind     string `json:"kind"`
				Name     string `json:"name"`
			} `json:"additionalRoles"`
			Subjects []struct {
				Kind      string `json:"kind"`
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"subjects"`
		} `json:"spec,omitempty"`
	}{
		{
			Name: "test-rule",
			Spec: struct {
				AccessLevel                   string             `json:"accessLevel"`
				PortForwarding                bool               `json:"portForwarding"`
				AllowScale                    bool               `json:"allowScale"`
				AllowAccessToSystemNamespaces bool               `json:"allowAccessToSystemNamespaces"`
				LimitNamespaces               []string           `json:"limitNamespaces"`
				NamespaceSelector             *NamespaceSelector `json:"namespaceSelector"`
				AdditionalRoles               []struct {
					APIGroup string `json:"apiGroup"`
					Kind     string `json:"kind"`
					Name     string `json:"name"`
				} `json:"additionalRoles"`
				Subjects []struct {
					Kind      string `json:"kind"`
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
				} `json:"subjects"`
			}{
				LimitNamespaces: []string{"allowed-ns"},
				Subjects: []struct {
					Kind      string `json:"kind"`
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
				}{
					{Kind: "User", Name: "testuser"},
				},
			},
		},
	}}

	writeConfig(t, configPath, config)

	fakeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)

	engine, err := NewEngine(
		configPath,
		informerFactory.Core().V1().Namespaces().Lister(),
		func() bool { return true },
		fakeClient.Discovery(),
	)
	require.NoError(t, err)

	// Should have an entry for testuser
	entries := engine.affectedDirs("testuser", nil)
	assert.Len(t, entries, 1, "should have one entry for testuser")
}

// TestEngine_SystemNamespaces tests restrictions on system namespaces
func TestEngine_SystemNamespaces(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Configuration WITHOUT access to system namespaces
	config := UserAuthzConfig{CRDs: []struct {
		Name string `json:"name"`
		Spec struct {
			AccessLevel                   string             `json:"accessLevel"`
			PortForwarding                bool               `json:"portForwarding"`
			AllowScale                    bool               `json:"allowScale"`
			AllowAccessToSystemNamespaces bool               `json:"allowAccessToSystemNamespaces"`
			LimitNamespaces               []string           `json:"limitNamespaces"`
			NamespaceSelector             *NamespaceSelector `json:"namespaceSelector"`
			AdditionalRoles               []struct {
				APIGroup string `json:"apiGroup"`
				Kind     string `json:"kind"`
				Name     string `json:"name"`
			} `json:"additionalRoles"`
			Subjects []struct {
				Kind      string `json:"kind"`
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"subjects"`
		} `json:"spec,omitempty"`
	}{
		{
			Name: "limited-user-rule",
			Spec: struct {
				AccessLevel                   string             `json:"accessLevel"`
				PortForwarding                bool               `json:"portForwarding"`
				AllowScale                    bool               `json:"allowScale"`
				AllowAccessToSystemNamespaces bool               `json:"allowAccessToSystemNamespaces"`
				LimitNamespaces               []string           `json:"limitNamespaces"`
				NamespaceSelector             *NamespaceSelector `json:"namespaceSelector"`
				AdditionalRoles               []struct {
					APIGroup string `json:"apiGroup"`
					Kind     string `json:"kind"`
					Name     string `json:"name"`
				} `json:"additionalRoles"`
				Subjects []struct {
					Kind      string `json:"kind"`
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
				} `json:"subjects"`
			}{
				AllowAccessToSystemNamespaces: false,
				Subjects: []struct {
					Kind      string `json:"kind"`
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
				}{
					{Kind: "User", Name: "limited-user"},
				},
			},
		},
	}}

	writeConfig(t, configPath, config)

	fakeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)

	engine, err := NewEngine(
		configPath,
		informerFactory.Core().V1().Namespaces().Lister(),
		func() bool { return true },
		fakeClient.Discovery(),
	)
	require.NoError(t, err)

	systemNamespaces := []string{"kube-system", "kube-public", "d8-system", "default"}

	for _, ns := range systemNamespaces {
		t.Run("system_namespace_"+ns, func(t *testing.T) {
			attrs := &testAttrs{
				user:            &user.DefaultInfo{Name: "limited-user"},
				verb:            "get",
				namespace:       ns,
				resource:        "pods",
				resourceRequest: true,
			}

			decision, reason, err := engine.Authorize(context.Background(), attrs)
			require.NoError(t, err)
			assert.Equal(t, authorizer.DecisionDeny, decision, "system namespace %s should be denied", ns)
			assert.Contains(t, reason, "no access")
		})
	}
}

// TestEngine_GroupBasedRules tests group-based authorization rules
func TestEngine_GroupBasedRules(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	config := UserAuthzConfig{CRDs: []struct {
		Name string `json:"name"`
		Spec struct {
			AccessLevel                   string             `json:"accessLevel"`
			PortForwarding                bool               `json:"portForwarding"`
			AllowScale                    bool               `json:"allowScale"`
			AllowAccessToSystemNamespaces bool               `json:"allowAccessToSystemNamespaces"`
			LimitNamespaces               []string           `json:"limitNamespaces"`
			NamespaceSelector             *NamespaceSelector `json:"namespaceSelector"`
			AdditionalRoles               []struct {
				APIGroup string `json:"apiGroup"`
				Kind     string `json:"kind"`
				Name     string `json:"name"`
			} `json:"additionalRoles"`
			Subjects []struct {
				Kind      string `json:"kind"`
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"subjects"`
		} `json:"spec,omitempty"`
	}{
		{
			Name: "developers-rule",
			Spec: struct {
				AccessLevel                   string             `json:"accessLevel"`
				PortForwarding                bool               `json:"portForwarding"`
				AllowScale                    bool               `json:"allowScale"`
				AllowAccessToSystemNamespaces bool               `json:"allowAccessToSystemNamespaces"`
				LimitNamespaces               []string           `json:"limitNamespaces"`
				NamespaceSelector             *NamespaceSelector `json:"namespaceSelector"`
				AdditionalRoles               []struct {
					APIGroup string `json:"apiGroup"`
					Kind     string `json:"kind"`
					Name     string `json:"name"`
				} `json:"additionalRoles"`
				Subjects []struct {
					Kind      string `json:"kind"`
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
				} `json:"subjects"`
			}{
				LimitNamespaces: []string{"dev-.*"},
				Subjects: []struct {
					Kind      string `json:"kind"`
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
				}{
					{Kind: "Group", Name: "developers"},
				},
			},
		},
	}}

	writeConfig(t, configPath, config)

	fakeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)

	engine, err := NewEngine(
		configPath,
		informerFactory.Core().V1().Namespaces().Lister(),
		func() bool { return true },
		fakeClient.Discovery(),
	)
	require.NoError(t, err)

	tests := []struct {
		name       string
		user       string
		groups     []string
		namespace  string
		expectDeny bool
	}{
		{"developer in dev-team", "alice", []string{"developers"}, "dev-team", false},
		{"developer in dev-frontend", "bob", []string{"developers"}, "dev-frontend", false},
		{"developer in prod", "alice", []string{"developers"}, "prod", true},
		{"non-developer in dev-team", "charlie", []string{"guests"}, "dev-team", false}, // no rule for guests
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := &testAttrs{
				user:            &user.DefaultInfo{Name: tt.user, Groups: tt.groups},
				verb:            "get",
				namespace:       tt.namespace,
				resource:        "pods",
				resourceRequest: true,
			}

			decision, _, err := engine.Authorize(context.Background(), attrs)
			require.NoError(t, err)

			if tt.expectDeny {
				assert.Equal(t, authorizer.DecisionDeny, decision)
			} else {
				assert.NotEqual(t, authorizer.DecisionDeny, decision)
			}
		})
	}
}

// TestEngine_ServiceAccountRules tests ServiceAccount authorization rules
func TestEngine_ServiceAccountRules(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	config := UserAuthzConfig{CRDs: []struct {
		Name string `json:"name"`
		Spec struct {
			AccessLevel                   string             `json:"accessLevel"`
			PortForwarding                bool               `json:"portForwarding"`
			AllowScale                    bool               `json:"allowScale"`
			AllowAccessToSystemNamespaces bool               `json:"allowAccessToSystemNamespaces"`
			LimitNamespaces               []string           `json:"limitNamespaces"`
			NamespaceSelector             *NamespaceSelector `json:"namespaceSelector"`
			AdditionalRoles               []struct {
				APIGroup string `json:"apiGroup"`
				Kind     string `json:"kind"`
				Name     string `json:"name"`
			} `json:"additionalRoles"`
			Subjects []struct {
				Kind      string `json:"kind"`
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"subjects"`
		} `json:"spec,omitempty"`
	}{
		{
			Name: "sa-rule",
			Spec: struct {
				AccessLevel                   string             `json:"accessLevel"`
				PortForwarding                bool               `json:"portForwarding"`
				AllowScale                    bool               `json:"allowScale"`
				AllowAccessToSystemNamespaces bool               `json:"allowAccessToSystemNamespaces"`
				LimitNamespaces               []string           `json:"limitNamespaces"`
				NamespaceSelector             *NamespaceSelector `json:"namespaceSelector"`
				AdditionalRoles               []struct {
					APIGroup string `json:"apiGroup"`
					Kind     string `json:"kind"`
					Name     string `json:"name"`
				} `json:"additionalRoles"`
				Subjects []struct {
					Kind      string `json:"kind"`
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
				} `json:"subjects"`
			}{
				LimitNamespaces: []string{"app-ns"},
				Subjects: []struct {
					Kind      string `json:"kind"`
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
				}{
					{Kind: "ServiceAccount", Name: "my-sa", Namespace: "app-ns"},
				},
			},
		},
	}}

	writeConfig(t, configPath, config)

	fakeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)

	engine, err := NewEngine(
		configPath,
		informerFactory.Core().V1().Namespaces().Lister(),
		func() bool { return true },
		fakeClient.Discovery(),
	)
	require.NoError(t, err)

	// ServiceAccount user format: system:serviceaccount:<namespace>:<name>
	saUser := "system:serviceaccount:app-ns:my-sa"

	tests := []struct {
		name       string
		namespace  string
		expectDeny bool
	}{
		{"SA in allowed namespace", "app-ns", false},
		{"SA in other namespace", "other-ns", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := &testAttrs{
				user:            &user.DefaultInfo{Name: saUser},
				verb:            "get",
				namespace:       tt.namespace,
				resource:        "pods",
				resourceRequest: true,
			}

			decision, _, err := engine.Authorize(context.Background(), attrs)
			require.NoError(t, err)

			if tt.expectDeny {
				assert.Equal(t, authorizer.DecisionDeny, decision)
			} else {
				assert.NotEqual(t, authorizer.DecisionDeny, decision)
			}
		})
	}
}

// TestEngine_MatchAnySelector tests MatchAny selector behavior
func TestEngine_MatchAnySelector(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Rule with MatchAny = true - allows all namespaces
	config := UserAuthzConfig{CRDs: []struct {
		Name string `json:"name"`
		Spec struct {
			AccessLevel                   string             `json:"accessLevel"`
			PortForwarding                bool               `json:"portForwarding"`
			AllowScale                    bool               `json:"allowScale"`
			AllowAccessToSystemNamespaces bool               `json:"allowAccessToSystemNamespaces"`
			LimitNamespaces               []string           `json:"limitNamespaces"`
			NamespaceSelector             *NamespaceSelector `json:"namespaceSelector"`
			AdditionalRoles               []struct {
				APIGroup string `json:"apiGroup"`
				Kind     string `json:"kind"`
				Name     string `json:"name"`
			} `json:"additionalRoles"`
			Subjects []struct {
				Kind      string `json:"kind"`
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"subjects"`
		} `json:"spec,omitempty"`
	}{
		{
			Name: "match-any-rule",
			Spec: struct {
				AccessLevel                   string             `json:"accessLevel"`
				PortForwarding                bool               `json:"portForwarding"`
				AllowScale                    bool               `json:"allowScale"`
				AllowAccessToSystemNamespaces bool               `json:"allowAccessToSystemNamespaces"`
				LimitNamespaces               []string           `json:"limitNamespaces"`
				NamespaceSelector             *NamespaceSelector `json:"namespaceSelector"`
				AdditionalRoles               []struct {
					APIGroup string `json:"apiGroup"`
					Kind     string `json:"kind"`
					Name     string `json:"name"`
				} `json:"additionalRoles"`
				Subjects []struct {
					Kind      string `json:"kind"`
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
				} `json:"subjects"`
			}{
				NamespaceSelector: &NamespaceSelector{
					MatchAny: true,
				},
				Subjects: []struct {
					Kind      string `json:"kind"`
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
				}{
					{Kind: "User", Name: "super-user"},
				},
			},
		},
	}}

	writeConfig(t, configPath, config)

	fakeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)

	engine, err := NewEngine(
		configPath,
		informerFactory.Core().V1().Namespaces().Lister(),
		func() bool { return true },
		fakeClient.Discovery(),
	)
	require.NoError(t, err)

	// With MatchAny=true user should not be blocked in any namespace
	attrs := &testAttrs{
		user:            &user.DefaultInfo{Name: "super-user"},
		verb:            "get",
		namespace:       "any-namespace",
		resource:        "pods",
		resourceRequest: true,
	}

	decision, _, err := engine.Authorize(context.Background(), attrs)
	require.NoError(t, err)
	assert.Equal(t, authorizer.DecisionNoOpinion, decision, "MatchAny should allow all namespaces")
}

// TestEngine_NonResourceRequestSkipped tests that non-resource requests are skipped
func TestEngine_NonResourceRequestSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	writeConfig(t, configPath, UserAuthzConfig{})

	fakeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)

	engine, err := NewEngine(
		configPath,
		informerFactory.Core().V1().Namespaces().Lister(),
		func() bool { return true },
		fakeClient.Discovery(),
	)
	require.NoError(t, err)

	attrs := &testAttrs{
		user:            &user.DefaultInfo{Name: "anyone"},
		verb:            "get",
		path:            "/healthz",
		resourceRequest: false,
	}

	decision, _, err := engine.Authorize(context.Background(), attrs)
	require.NoError(t, err)
	assert.Equal(t, authorizer.DecisionNoOpinion, decision, "non-resource requests should be skipped")
}

// === Helpers ===

func writeConfig(t *testing.T, path string, config UserAuthzConfig) {
	data, err := json.Marshal(config)
	require.NoError(t, err)
	err = os.WriteFile(path, data, 0644)
	require.NoError(t, err)
}

type testAttrs struct {
	user            user.Info
	verb            string
	namespace       string
	resource        string
	subresource     string
	name            string
	apiGroup        string
	apiVersion      string
	path            string
	resourceRequest bool
}

func (t *testAttrs) GetUser() user.Info { return t.user }
func (t *testAttrs) GetVerb() string    { return t.verb }
func (t *testAttrs) IsReadOnly() bool {
	return t.verb == "get" || t.verb == "list" || t.verb == "watch"
}
func (t *testAttrs) GetNamespace() string    { return t.namespace }
func (t *testAttrs) GetResource() string     { return t.resource }
func (t *testAttrs) GetSubresource() string  { return t.subresource }
func (t *testAttrs) GetName() string         { return t.name }
func (t *testAttrs) GetAPIGroup() string     { return t.apiGroup }
func (t *testAttrs) GetAPIVersion() string   { return t.apiVersion }
func (t *testAttrs) IsResourceRequest() bool { return t.resourceRequest }
func (t *testAttrs) GetPath() string         { return t.path }
