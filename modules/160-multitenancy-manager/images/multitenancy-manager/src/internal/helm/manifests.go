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

package helm

import (
	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
)

// RenderManifests renders the manifests text of a project template for the project, as a
// resourcesTemplate is rendered: the same chart, the same values (the project name, the template
// name, and the parameters completed with the defaults of parametersSchema), and the same refusal of
// a parameter that becomes manifest structure instead of a value (see
// ensureRenderedParametersStayValues). The check runs on every render rather than only before an
// apply, as for a Helm release: the objects are applied with server-side apply, which has no release
// to tell an unchanged render by.
//
// The render has no cluster behind it, so `lookup` finds nothing: the text is applied as a user of
// the project, and a read with the rights of the controller would hand it what that user cannot see.
// Nothing here applies the objects either: the caller does, as that user.
func (c *Client) RenderManifests(project *v1alpha3.Project, manifests string, parametersSchema map[string]any) (string, error) {
	template := new(v1alpha1.ProjectTemplate)
	template.Name = project.Spec.ProjectTemplateName
	template.Spec.ResourcesTemplate = manifests
	template.Spec.ParametersSchema.OpenAPIV3Schema = parametersSchema

	parameters := templateParameters(project, template)
	rendered, err := c.renderTemplate(project, template, parameters)
	if err != nil {
		return "", err
	}
	if err := c.ensureRenderedParametersStayValues(project, template, parameters, rendered); err != nil {
		return "", err
	}
	return rendered, nil
}
