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

package project

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha2"
)

// isStructured reports whether a schema-based (v1alpha2) ProjectTemplate is rendered natively from its
// structured fields (controller/internal/render). A template that still carries a Helm resourcesTemplate
// string is rendered through the legacy helm engine path instead; both are supported per ADR-3, with
// resourcesTemplate kept as a deprecated escape hatch.
func isStructured(t *v1alpha2.ProjectTemplate) bool {
	//nolint:staticcheck // SA1019: reading the deprecated escape-hatch field is how we pick the render path.
	return strings.TrimSpace(t.Spec.ResourcesTemplate) == ""
}

// legacyTemplate projects a v1alpha2 ProjectTemplate onto the v1alpha1 shape used for two purposes:
// validating Project.spec.parameters against the parametersSchema (both paths) and rendering the
// legacy resourcesTemplate through the helm engine (resourcesTemplate path only). Structured fields
// are intentionally dropped: they never feed the helm engine — they are rendered natively.
func legacyTemplate(t *v1alpha2.ProjectTemplate) *v1alpha1.ProjectTemplate {
	return &v1alpha1.ProjectTemplate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       v1alpha1.ProjectTemplateKind,
		},
		ObjectMeta: *t.ObjectMeta.DeepCopy(),
		Spec: v1alpha1.ProjectTemplateSpec{
			Description: t.Spec.Description,
			//nolint:staticcheck // SA1019: the legacy render path requires copying the deprecated field verbatim.
			ResourcesTemplate: t.Spec.ResourcesTemplate,
			ParametersSchema:  v1alpha1.ParametersSchema{OpenAPIV3Schema: t.Spec.ParametersSchema.OpenAPIV3Schema},
		},
	}
}
