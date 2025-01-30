package schema

import "github.com/go-openapi/spec"

type SchemaTransformer interface {
	Transform(s *spec.Schema) *spec.Schema
}

func TransformSchema(s *spec.Schema, transformers ...SchemaTransformer) *spec.Schema {
	for _, transformer := range transformers {
		s = transformer.Transform(s)
	}
	return s
}
