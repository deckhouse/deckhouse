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
