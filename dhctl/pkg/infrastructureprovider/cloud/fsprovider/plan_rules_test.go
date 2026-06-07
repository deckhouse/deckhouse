// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fsprovider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func TestLoadPlanRules_FileAbsent(t *testing.T) {
	dir := t.TempDir()

	rules, err := loadPlanRules(filepath.Join(dir, "terraform_versions.yml"))
	require.NoError(t, err)
	require.Nil(t, rules)
}

func TestLoadPlanRules_ValidFile(t *testing.T) {
	dir := t.TempDir()
	infraVersionsFile := filepath.Join(dir, "terraform_versions.yml")
	planRulesFile := filepath.Join(dir, "plan_rules.yml")

	require.NoError(t, os.WriteFile(planRulesFile, []byte(`
kubernetes:
  vmChange:
    resourceType: kubernetes_manifest
    fieldEquals:
      path: manifest.kind
      value: VirtualMachine
`), 0o644))

	rules, err := loadPlanRules(infraVersionsFile)
	require.NoError(t, err)
	require.Len(t, rules, 1)

	rule := rules["kubernetes"]
	require.NotNil(t, rule)
	require.Equal(t, "kubernetes_manifest", rule.ResourceType)
	require.NotNil(t, rule.FieldEquals)
	require.Equal(t, "manifest.kind", rule.FieldEquals.Path)
	require.Equal(t, "VirtualMachine", rule.FieldEquals.Value)
}

func TestLoadPlanRules_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	infraVersionsFile := filepath.Join(dir, "terraform_versions.yml")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plan_rules.yml"), []byte("not: [valid"), 0o644))

	_, err := loadPlanRules(infraVersionsFile)
	require.Error(t, err)
}

func TestLoadSettings_DVPRequiresPlanRules(t *testing.T) {
	dir := t.TempDir()
	infraVersionsFile := filepath.Join(dir, "terraform_versions.yml")

	require.NoError(t, os.WriteFile(infraVersionsFile, []byte(`
opentofu: 1.12.0
terraform: 0.14.8
kubernetes:
  namespace: hashicorp
  cloudName: DVP
  type: kubernetes
  version: "2.38.0"
  artifact: terraform-provider-kubernetes
  artifactBinary: terraform-provider-kubernetes
  destinationBinary: terraform-provider-kubernetes
  vmResourceType: kubernetes_manifest
  useOpentofu: true
`), 0o644))

	_, err := loadTerraformVersionFileSettings(infraVersionsFile, log.GetDefaultLogger())
	require.Error(t, err)
	require.Contains(t, err.Error(), "DVP")
	require.Contains(t, err.Error(), "plan_rules.yml")
}

func TestLoadSettings_DVPWithPlanRulesSucceeds(t *testing.T) {
	dir := t.TempDir()
	infraVersionsFile := filepath.Join(dir, "terraform_versions.yml")

	require.NoError(t, os.WriteFile(infraVersionsFile, []byte(`
opentofu: 1.12.0
terraform: 0.14.8
kubernetes:
  namespace: hashicorp
  cloudName: DVP
  type: kubernetes
  version: "2.38.0"
  artifact: terraform-provider-kubernetes
  artifactBinary: terraform-provider-kubernetes
  destinationBinary: terraform-provider-kubernetes
  vmResourceType: kubernetes_manifest
  useOpentofu: true
`), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "plan_rules.yml"), []byte(`
dvp:
  vmChange:
    resourceType: kubernetes_manifest
    fieldEquals:
      path: manifest.kind
      value: VirtualMachine
`), 0o644))

	store, err := loadTerraformVersionFileSettings(infraVersionsFile, log.GetDefaultLogger())
	require.NoError(t, err)
	require.NotNil(t, store["dvp"].VMChange())
}
