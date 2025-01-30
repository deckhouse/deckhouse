package schema

import "github.com/go-openapi/spec"

type AdditionalPropertiesTransformer struct {
	Parent *spec.Schema
}

// Transform sets undefined AdditionalProperties to false recursively.
func (t *AdditionalPropertiesTransformer) Transform(s *spec.Schema) *spec.Schema {
	if s == nil {
		return nil
	}

	if s.AdditionalProperties == nil {
		s.AdditionalProperties = &spec.SchemaOrBool{
			Allows: false,
		}
	}

	for k, prop := range s.Properties {
		if prop.AdditionalProperties == nil {
			prop.AdditionalProperties = &spec.SchemaOrBool{
				Allows: false,
			}
			ts := prop
			s.Properties[k] = *t.Transform(&ts)
		}
	}

	if s.Items != nil {
		if s.Items.Schema != nil {
			s.Items.Schema = t.Transform(s.Items.Schema)
		}
		for i, item := range s.Items.Schemas {
			ts := item
			s.Items.Schemas[i] = *t.Transform(&ts)
		}
	}

	return s
}
