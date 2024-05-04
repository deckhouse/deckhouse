package widget

import (
	"strconv"

	"github.com/go-openapi/spec"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/validate"
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
	d, err := validate.OpenAPIValidate(f.schema, f.data)
	if err != nil {
		return err
	}

	f.data = d.(map[string]interface{})

	return nil
}

func (f *OpenAPIForm) Data() map[string]interface{} {
	return f.data
}
