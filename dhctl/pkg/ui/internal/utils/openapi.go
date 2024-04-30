package utils

import (
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"github.com/go-openapi/validate/post"
	"github.com/hashicorp/go-multierror"
)

func OpenAPIValidate(schema *spec.Schema, data interface{}) (interface{}, error) {
	validator := validate.NewSchemaValidator(schema, nil, "", strfmt.Default)

	result := validator.Validate(data)
	if result.IsValid() {
		// Add default values from openAPISpec
		post.ApplyDefaults(result)

		return result.Data(), nil
	}

	var allErrs *multierror.Error
	allErrs = multierror.Append(allErrs, result.Errors...)

	return nil, allErrs.ErrorOrNil()
}
