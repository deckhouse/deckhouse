// Copyright 2026 Flant JSC
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

package config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-openapi/spec"
	"sigs.k8s.io/yaml"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

// xDocDeprecatedExtension is the vendor extension openapi schemas use to mark
// a field as deprecated. It is the same marker the docs generator reads
// (see pkg/crd-enricher and candi/openapi/*.yaml), so any field already
// documented as deprecated is picked up here for free.
const xDocDeprecatedExtension = "x-doc-deprecated"

// warnDeprecatedFields logs a warning for every field actually set in doc
// whose schema node carries "x-doc-deprecated: true". It works for any
// document validated against a schema from the SchemaStore -
// ClusterConfiguration, InitConfiguration, StaticClusterConfiguration,
// provider-specific cluster configurations, ModuleConfig settings, and so on -
// so a field only needs to be marked deprecated in its openapi schema to
// start warning users, with no dhctl code change required.
//
// name is the document's metadata.name, if it has one (e.g. a ModuleConfig),
// so the warning identifies which resource is affected; it is empty for
// documents that don't carry a metadata.name, such as ClusterConfiguration.
func warnDeprecatedFields(ctx context.Context, index *SchemaIndex, name string, doc json.RawMessage, schema *spec.Schema) {
	warnDeprecatedProperties(ctx, index, name, "", doc, schema)
}

// extractMetadataName reads metadata.name out of a raw document, or returns
// an empty string if the document has none.
func extractMetadataName(doc []byte) string {
	var idx namedIndex
	if err := yaml.Unmarshal(doc, &idx); err != nil {
		return ""
	}
	return idx.Metadata.Name
}

func warnDeprecatedProperties(ctx context.Context, index *SchemaIndex, name, pathPrefix string, doc json.RawMessage, schema *spec.Schema) {
	if schema == nil || len(doc) == 0 {
		return
	}

	if len(schema.Properties) > 0 {
		var properties map[string]json.RawMessage
		if err := yaml.Unmarshal(doc, &properties); err != nil {
			return
		}

		for field, fieldSchema := range schema.Properties {
			raw, ok := properties[field]
			if !ok {
				continue
			}

			path := joinFieldPath(pathPrefix, field)

			if deprecated, _ := fieldSchema.Extensions.GetBool(xDocDeprecatedExtension); deprecated {
				warnDeprecatedField(ctx, index, name, path)
			}

			warnDeprecatedProperties(ctx, index, name, path, raw, &fieldSchema)
		}
	}

	if itemSchema := schema.Items; itemSchema != nil && itemSchema.Schema != nil {
		var items []json.RawMessage
		if err := yaml.Unmarshal(doc, &items); err != nil {
			return
		}

		for i, item := range items {
			warnDeprecatedProperties(ctx, index, name, fmt.Sprintf("%s[%d]", pathPrefix, i), item, itemSchema.Schema)
		}
	}
}

func joinFieldPath(prefix, field string) string {
	if prefix == "" {
		return field
	}
	return prefix + "." + field
}

func warnDeprecatedField(ctx context.Context, index *SchemaIndex, name, path string) {
	logger := dhlog.FromContext(ctx)
	logger.WarnContext(ctx, "=================================================================")
	logger.WarnContext(ctx, fmt.Sprintf("DEPRECATED: %q in %s is deprecated.", path, kindLabel(index, name)))
	logger.WarnContext(ctx, "Support for this field will be removed in a future release.")
	logger.WarnContext(ctx, "=================================================================")
}

// kindLabel renders "Kind" or, when the document has a metadata.name, `Kind "name"`.
func kindLabel(index *SchemaIndex, name string) string {
	if name == "" {
		return index.Kind
	}
	return fmt.Sprintf("%s %q", index.Kind, name)
}
