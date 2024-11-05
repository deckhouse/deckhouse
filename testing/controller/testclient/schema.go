// Copyright 2024 Flant JSC
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

package testclient

import (
	"fmt"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

func addValidator(
	crd apiextensionsv1.CustomResourceDefinition,
	validators map[schema.GroupVersionKind]validation.SchemaValidator,
	schemaMap map[string]*spec.Schema,
) error {
	var apiServerCRD apiextensions.CustomResourceDefinition

	err := apiextensionsv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(&crd, &apiServerCRD, nil)
	if err != nil {
		return fmt.Errorf("convert %s to apiserver crd: %w", crd.Name, err)
	}

	for _, ver := range apiServerCRD.Spec.Versions {
		s, err := apiextensions.GetSchemaForVersion(&apiServerCRD, ver.Name)
		if err != nil {
			return fmt.Errorf("get schema for %s.%s: %w", ver.Name, apiServerCRD.Name, err)
		}

		validator, _, err := validation.NewSchemaValidator(s.OpenAPIV3Schema)
		if err != nil {
			return fmt.Errorf("new schema validator from %s.%s %w", ver.Name, apiServerCRD.Name, err)
		}

		gvk := schema.GroupVersionKind{
			Group:   crd.Spec.Group,
			Version: ver.Name,
			Kind:    crd.Spec.Names.Kind,
		}
		openAPITypes := new(spec.Schema)
		err = validation.ConvertJSONSchemaProps(s.OpenAPIV3Schema, openAPITypes)
		if err != nil {
			return fmt.Errorf("convert JSON schema props: %w", err)
		}
		schemaMap[gvk.String()] = openAPITypes

		validators[schema.GroupVersionKind{
			Group:   apiServerCRD.Spec.Group,
			Version: ver.Name,
			Kind:    apiServerCRD.Spec.Names.Kind,
		}] = validator
	}
	return nil
}
