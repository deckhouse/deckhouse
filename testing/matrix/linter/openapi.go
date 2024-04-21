/*
Copyright 2021 Flant JSC

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

package linter

import (
	"fmt"

	"github.com/iancoleman/strcase"

	"github.com/gammazero/deque"
	"github.com/go-openapi/spec"
	"github.com/mohae/deepcopy"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/deckhouse/testing/library"
	"github.com/deckhouse/deckhouse/testing/library/values_validation"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/utils"
)

const (
	ExamplesKey = "x-examples"
	ArrayObject = "array"
	ObjectKey   = "object"
)

func helmFormatModuleImages(m utils.Module, rawValues []interface{}) ([]chartutil.Values, error) {
	caps := chartutil.DefaultCapabilities
	vers := []string(caps.APIVersions)
	vers = append(vers, "autoscaling.k8s.io/v1/VerticalPodAutoscaler")
	caps.APIVersions = vers

	digests, err := library.GetModulesImagesDigests(m.Path)
	if err != nil {
		return nil, err
	}

	values := make([]chartutil.Values, 0, len(rawValues))
	for _, singleValue := range rawValues {
		applyDigests(digests, singleValue)

		top := map[string]interface{}{
			"Chart":        m.Chart.Metadata,
			"Capabilities": caps,
			"Release": map[string]interface{}{
				"Name":      m.Name,
				"Namespace": m.Namespace,
				"IsUpgrade": true,
				"IsInstall": true,
				"Revision":  0,
				"Service":   "Helm",
			},
			"Values": singleValue,
		}

		values = append(values, top)
	}
	return values, nil
}

func ComposeValuesFromSchemas(m utils.Module) ([]chartutil.Values, error) {
	// TODO(maksim.nabokikh): Move the code from below after migrating from values matrix to load schemas only once.
	valueValidator, err := values_validation.NewValuesValidator(m.Name, m.Path)
	if err != nil {
		return nil, fmt.Errorf("schemas load: %v", err)
	}

	camelizedModuleName := strcase.ToLowerCamel(m.Name)

	values := valueValidator.ModuleSchemaStorage.Schemas["values"]
	if values == nil {
		return nil, fmt.Errorf("cannot find openapi values schema for module %s", m.Name)
	}

	moduleSchema := *values
	moduleSchema.Default = make(map[string]interface{})

	globalSchema := *valueValidator.GlobalSchemaStorage.Schemas["values"]
	globalSchema.Default = make(map[string]interface{})

	combinedSchema := spec.Schema{}
	combinedSchema.Properties = map[string]spec.Schema{camelizedModuleName: moduleSchema, "global": globalSchema}

	rawValues, err := NewOpenAPIValuesGenerator(&combinedSchema).Do()
	if err != nil {
		return nil, fmt.Errorf("generate vlues: %v", err)
	}

	return helmFormatModuleImages(m, rawValues)
}

func mergeSchemas(rootSchema spec.Schema, schemas ...spec.Schema) spec.Schema {
	rootSchema.OneOf = nil
	rootSchema.AllOf = nil
	rootSchema.AnyOf = nil

	for _, schema := range schemas {
		for key, prop := range schema.Properties {
			rootSchema.Properties[key] = prop
		}
		rootSchema.OneOf = schema.OneOf
		rootSchema.AllOf = schema.AllOf
		rootSchema.AnyOf = schema.AnyOf
	}

	return rootSchema
}

type SchemaNode struct {
	Schema *spec.Schema

	Leaf *map[string]interface{}
}

type InteractionsCounter struct {
	counter int
}

func (c *InteractionsCounter) Inc() {
	c.counter++
}

func (c *InteractionsCounter) Zero() bool {
	return c.counter == 0
}

type OpenAPIValuesGenerator struct {
	rootSchema *spec.Schema

	schemaQueue *deque.Deque
	resultQueue *deque.Deque
}

func NewOpenAPIValuesGenerator(schema *spec.Schema) *OpenAPIValuesGenerator {
	s := deque.Deque{}
	r := deque.Deque{}

	return &OpenAPIValuesGenerator{
		rootSchema:  schema,
		schemaQueue: &s,
		resultQueue: &r,
	}
}

func (g *OpenAPIValuesGenerator) Do() ([]interface{}, error) {
	newItem := make(map[string]interface{})
	g.schemaQueue.PushBack(SchemaNode{Schema: g.rootSchema, Leaf: &newItem})

	for g.schemaQueue.Len() > 0 {
		tempNode := g.schemaQueue.PopFront().(SchemaNode)
		counter := InteractionsCounter{}

		err := g.parseProperties(&tempNode, &counter)
		if err != nil {
			return nil, err
		}
		if counter.Zero() {
			g.resultQueue.PushBack(tempNode)
		}
	}

	values := make([]interface{}, 0, g.resultQueue.Len())
	for g.resultQueue.Len() > 0 {
		resultNode := g.resultQueue.PopFront().(SchemaNode)
		values = append(values, *resultNode.Leaf)
	}

	return values, nil
}

func (g *OpenAPIValuesGenerator) pushBackNodesFromValues(tempNode *SchemaNode, key string, items []interface{}, counter *InteractionsCounter) {
	for _, item := range items {
		headNode := copyNode(tempNode, key, item)
		g.deleteNodeAndPushBack(&headNode, key, counter)
	}
}

func (g *OpenAPIValuesGenerator) generateAndPushBackNodes(tempNode *SchemaNode, key string, prop spec.Schema, counter *InteractionsCounter) error {
	downwardSchema := deepcopy.Copy(prop).(spec.Schema)
	// Recursive call, consider switching to a better solution.
	values, err := NewOpenAPIValuesGenerator(&downwardSchema).Do()
	if err != nil {
		return err
	}

	g.pushBackNodesFromValues(tempNode, key, values, counter)
	return nil
}

func (g *OpenAPIValuesGenerator) parseProperties(tempNode *SchemaNode, counter *InteractionsCounter) error {
	for key, prop := range tempNode.Schema.Properties {
		switch {
		case prop.Extensions[ExamplesKey] != nil:
			examples := prop.Extensions[ExamplesKey].([]interface{})
			g.pushBackNodesFromValues(tempNode, key, examples, counter)
			return nil

		case len(prop.Enum) > 0:
			g.pushBackNodesFromValues(tempNode, key, prop.Enum, counter)
			return nil

		case prop.Type.Contains(ObjectKey):
			if prop.Default == nil {
				g.deleteNodeAndPushBack(tempNode, key, counter)
				return nil
			}
			return g.generateAndPushBackNodes(tempNode, key, prop, counter)

		case prop.Default != nil:
			g.schemaQueue.PushBack(copyNode(tempNode, key, prop.Default))
			counter.Inc()
			return nil

		case prop.Type.Contains(ArrayObject) && prop.Items.Schema != nil:
			switch {
			case prop.Items.Schema.Default != nil:
				var wrapped []interface{}
				wrapped = append(wrapped, prop.Items.Schema.Default)

				g.schemaQueue.PushBack(copyNode(tempNode, key, wrapped))
				counter.Inc()
				return nil
			case prop.Items.Schema.Type.Contains(ObjectKey):
				if prop.Items.Schema.Default == nil {
					g.deleteNodeAndPushBack(tempNode, key, counter)
					return nil
				}

				downwardSchema := deepcopy.Copy(prop.Items.Schema).(spec.Schema)
				// Recursive call, consider switching to a better solution.
				values, err := NewOpenAPIValuesGenerator(&downwardSchema).Do()
				if err != nil {
					return err
				}

				for index, value := range values {
					var wrapped []interface{}
					wrapped = append(wrapped, value)

					values[index] = wrapped
				}
				g.pushBackNodesFromValues(tempNode, key, values, counter)
				return nil
			default:
				g.deleteNodeAndPushBack(tempNode, key, counter)
				return nil
			}
		case prop.AllOf != nil:
			// not implemented
			continue
		case prop.OneOf != nil:
			for _, schema := range prop.OneOf {
				downwardSchema := deepcopy.Copy(prop).(spec.Schema)

				mergedSchema := mergeSchemas(downwardSchema, schema)
				return g.generateAndPushBackNodes(tempNode, key, mergedSchema, counter)
			}
			return nil

		case prop.AnyOf != nil:
			for _, schema := range prop.AnyOf {
				downwardSchema := deepcopy.Copy(prop).(spec.Schema)
				mergedSchema := mergeSchemas(downwardSchema, schema)

				if err := g.generateAndPushBackNodes(tempNode, key, mergedSchema, counter); err != nil {
					return err
				}
			}
			return g.generateAndPushBackNodes(tempNode, key, prop, counter)
		default:
			g.deleteNodeAndPushBack(tempNode, key, counter)
			return nil
		}
	}
	return nil
}

func (g *OpenAPIValuesGenerator) deleteNodeAndPushBack(tempNode *SchemaNode, key string, counter *InteractionsCounter) {
	delete(tempNode.Schema.Properties, key)

	g.schemaQueue.PushBack(*tempNode)
	counter.Inc()
}

func copyNode(previousNode *SchemaNode, key string, value interface{}) SchemaNode {
	tempNode := *previousNode

	newSchema := deepcopy.Copy(*previousNode.Schema).(spec.Schema)
	delete(newSchema.Properties, key)

	leaf := *tempNode.Leaf
	leaf[key] = value

	newItem := deepcopy.Copy(leaf).(map[string]interface{})
	return SchemaNode{Leaf: &newItem, Schema: &newSchema}
}
