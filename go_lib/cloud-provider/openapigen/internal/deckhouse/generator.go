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

package deckhouse

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3gen"
	ctmarkers "sigs.k8s.io/controller-tools/pkg/markers"
)

func BuildSchema(root any, reg *ctmarkers.Registry) (*openapi3.Schema, error) {
	markersCustomizer, err := buildMarkersSchemaCustomizer(root, reg)
	if err != nil {
		return nil, fmt.Errorf("error creating markers schema customizer: %w", err)
	}

	options := []openapi3gen.Option{
		openapi3gen.UseAllExportedFields(),
		openapi3gen.ThrowErrorOnCycle(),
		openapi3gen.SchemaCustomizer(markersCustomizer),
	}

	ref, err := openapi3gen.NewSchemaRefForValue(root, nil, options...)
	if err != nil {
		return nil, fmt.Errorf("error creating schema ref for value: %w", err)
	}

	return ref.Value, nil
}
