/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// Package mttest builds multi-tenancy rules providers for tests: from rules written out in Go, and
// from the legacy user-authz config.json the tests of this apiserver were written against.
package mttest

import (
	"encoding/json"
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/user-authz/binding"
	"github.com/deckhouse/deckhouse/go_lib/user-authz/rules"
)

// Static is a rules provider over a directory built once. A nil directory stands for a provider
// that has not listed the rules yet.
type Static struct {
	dir *rules.Directory
}

// Directory implements multitenancy.RulesProvider.
func (s Static) Directory() *rules.Directory { return s.dir }

// HasSynced implements multitenancy.RulesProvider.
func (s Static) HasSynced() bool { return s.dir != nil }

// Rules builds a provider from rules.
func Rules(rs ...rules.Rule) Static {
	dir, _ := rules.NewBuilder().Build(rs)
	return Static{dir: dir}
}

// Unsynced is a provider whose rules have not been listed yet.
func Unsynced() Static { return Static{} }

// NoBindings is an empty index of rule bindings: nobody is bound by a rule the directory may not
// know.
func NoBindings() *binding.Index { return binding.NewIndex() }

// legacyConfig is the user-authz webhook config.json: the ClusterAuthorizationRules of a cluster as
// the module's hooks used to render them. Only the multi-tenancy fields matter; the namespaced
// AuthorizationRules under "ars" are ignored, as the webhook always ignored them.
type legacyConfig struct {
	CRDs []struct {
		Name string `json:"name"`
		Spec struct {
			AllowAccessToSystemNamespaces bool                     `json:"allowAccessToSystemNamespaces"`
			LimitNamespaces               []string                 `json:"limitNamespaces"`
			NamespaceSelector             *rules.NamespaceSelector `json:"namespaceSelector"`
			Subjects                      []rules.Subject          `json:"subjects"`
		} `json:"spec"`
	} `json:"crds"`
}

// ParseLegacyJSON turns a config.json body into rules.
func ParseLegacyJSON(body string) ([]rules.Rule, error) {
	var config legacyConfig
	if err := json.Unmarshal([]byte(body), &config); err != nil {
		return nil, err
	}
	rs := make([]rules.Rule, 0, len(config.CRDs))
	for _, crd := range config.CRDs {
		rs = append(rs, rules.Rule{
			Name:                          crd.Name,
			Subjects:                      crd.Spec.Subjects,
			LimitNamespaces:               crd.Spec.LimitNamespaces,
			NamespaceSelector:             crd.Spec.NamespaceSelector,
			AllowAccessToSystemNamespaces: crd.Spec.AllowAccessToSystemNamespaces,
		})
	}
	return rs, nil
}

// LegacyJSON builds a provider from a config.json body and fails the test when the body does not
// parse.
func LegacyJSON(tb testing.TB, body string) Static {
	tb.Helper()
	rs, err := ParseLegacyJSON(body)
	if err != nil {
		tb.Fatalf("parse legacy user-authz config: %v", err)
	}
	return Rules(rs...)
}
