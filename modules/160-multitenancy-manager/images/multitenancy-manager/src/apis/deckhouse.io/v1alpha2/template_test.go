/*
Copyright 2026 Flant JSC

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

package v1alpha2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	grantsv1alpha1 "controller/api/v1alpha1"
)

func TestHasResourcesTemplate(t *testing.T) {
	t.Parallel()

	assert.False(t, (&ProjectTemplateSpec{}).HasResourcesTemplate())
	assert.False(t, (&ProjectTemplateSpec{ResourcesTemplate: " \n\t"}).HasResourcesTemplate(), "a blank text is none")
	assert.True(t, (&ProjectTemplateSpec{ResourcesTemplate: "---\n"}).HasResourcesTemplate())
}

func TestHasManifests(t *testing.T) {
	t.Parallel()

	assert.False(t, (&ProjectTemplateSpec{}).HasManifests())
	assert.False(t, (&ProjectTemplateSpec{Manifests: " \n\t"}).HasManifests(), "a blank text is none")
	assert.True(t, (&ProjectTemplateSpec{Manifests: "---\n"}).HasManifests())
}

// Every field that renders an object is named, by its content rather than by its presence, and the
// ones only v1alpha2 has add the title and the grant fields.
func TestObjectFieldsAndV1alpha2OnlyFields(t *testing.T) {
	t.Parallel()

	var every ProjectTemplateSpec
	require.NoError(t, json.Unmarshal([]byte(`{
		"title": "Every field",
		"description": "a template",
		"resources": [{"resourceName": "storageclasses"}],
		"grantPolicies": ["library"],
		"podSecurityStandard": "Baseline",
		"networkPolicy": {"mode": "Isolated"},
		"nodeSelector": {"role": "project"},
		"tolerations": [{"key": "dedicated", "operator": "Exists"}],
		"namespaceMetadata": {"annotations": {"owner": "team"}},
		"features": {"monitoring": true},
		"logShipping": {"clusterDestinationRef": "loki"},
		"allowedUIDs": {"min": 1000, "max": 2000},
		"allowedGIDs": {"fromParam": "allowedGIDs"},
		"runtimeAudit": {"enabled": true},
		"parametersSchema": {"openAPIV3Schema": {"type": "object"}},
		"manifests": "---\n",
		"resourcesTemplate": "---\n"
	}`), &every))

	objectFields := []string{
		"podSecurityStandard", "networkPolicy", "nodeSelector", "tolerations", "namespaceMetadata",
		"features", "logShipping", "allowedUIDs", "allowedGIDs", "runtimeAudit", "manifests",
	}
	assert.Equal(t, objectFields, every.ObjectFields())
	assert.Equal(t, append([]string{"title", "resources", "grantPolicies"}, objectFields...), every.V1alpha2OnlyFields())

	emptyStanzas := ProjectTemplateSpec{
		Description:       "only what v1alpha1 has",
		NetworkPolicy:     &NetworkPolicySpec{},
		NamespaceMetadata: &NamespaceMetadata{},
		Features:          &FeaturesSpec{},
		LogShipping:       &LogShippingSpec{},
		RuntimeAudit:      &RuntimeAuditSpec{},
		Manifests:         " \n",
		ResourcesTemplate: "---\n",
	}
	assert.Empty(t, emptyStanzas.ObjectFields(), "an empty stanza and blank manifests render nothing")
	assert.Empty(t, emptyStanzas.V1alpha2OnlyFields())

	grantsOnly := ProjectTemplateSpec{Resources: []grantsv1alpha1.GrantResource{{ResourceName: "storageclasses"}}}
	assert.Empty(t, grantsOnly.ObjectFields())
	assert.Equal(t, []string{"resources"}, grantsOnly.V1alpha2OnlyFields())
}
