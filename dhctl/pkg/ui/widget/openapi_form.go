package widget

import (
	"strconv"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"github.com/go-openapi/validate/post"
	"github.com/hashicorp/go-multierror"
	"github.com/rivo/tview"
)

type OpenAPIForm struct {
	*tview.Form

	schema *spec.Schema
	data   map[string]interface{}
}

func NewOpenapiForm(schema *spec.Schema, fieldsWidth int) *OpenAPIForm {
	f := tview.NewForm()
	form := &OpenAPIForm{
		Form:   f,
		schema: schema,
		data:   make(map[string]interface{}),
	}

	for prop, schemaProp := range schema.SchemaProps.Properties {
		t := schemaProp.Type
		prop := prop
		switch t[0] {
		case "string":
			d := ""
			if schemaProp.SchemaProps.Default != nil {
				d = schemaProp.SchemaProps.Default.(string)
			}
			val, found := schemaProp.Extensions.GetString("x-ui-multiline")
			if found {
				rows, err := strconv.Atoi(val)
				if err != nil {
					panic(err)
				}

				form.AddTextArea(prop, d, fieldsWidth, rows, 0, func(text string) {
					form.data[prop] = text
				})

				continue
			}

			form.AddInputField(prop, d, fieldsWidth, nil, func(text string) {
				form.data[prop] = text
			})
		case "boolean":
			d := false
			if schemaProp.SchemaProps.Default != nil {
				d = schemaProp.SchemaProps.Default.(bool)
			}
			form.AddCheckbox(prop, d, func(checked bool) {
				form.data[prop] = checked
			})
		}
	}

	return form
}

func (f *OpenAPIForm) Validate() error {
	validator := validate.NewSchemaValidator(f.schema, nil, "", strfmt.Default)

	result := validator.Validate(f.data)
	if result.IsValid() {
		// Add default values from openAPISpec
		post.ApplyDefaults(result)
		f.data = result.Data().(map[string]interface{})

		return nil
	}

	var allErrs *multierror.Error
	allErrs = multierror.Append(allErrs, result.Errors...)

	return allErrs.ErrorOrNil()
}

func (f *OpenAPIForm) Data() map[string]interface{} {
	return f.data
}
