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
	"sort"
	"sync"

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
	if c := collectorFromContext(ctx); c != nil {
		c.add(kindLabel(index, name), path)
		return
	}

	// No collection scope open (a one-off Validate from a caller that does not parse a whole
	// config): report the single field right away rather than dropping it.
	c := &deprecationCollector{}
	c.add(kindLabel(index, name), path)
	c.flush(ctx)
}

// deprecationCollector accumulates the deprecated fields found while a whole configuration is
// parsed, so they are reported as one block instead of a four-line banner per field. Parsing a
// cluster config of any size otherwise buries the rest of the output under repeated banners.
//
// It is safe for concurrent use: validation of independent documents may run on several
// goroutines.
type deprecationCollector struct {
	mu sync.Mutex

	// order preserves the order in which kinds were first seen, so the report is stable
	// (map iteration is not) and follows the document order of the config.
	order []string
	// fields maps a kind label to the deprecated field paths found in it.
	fields map[string][]string
	// seen deduplicates kind+path pairs: the same document is validated more than once in a
	// single run (registry pre-parse, retries of the read-from-cluster loop), and each pass
	// would otherwise add the same field again.
	seen map[string]struct{}
}

type deprecationCollectorKey struct{}

// withDeprecationCollector opens a collection scope: deprecated fields found under the returned
// context are accumulated and reported by the returned flush function as a single block.
//
// Scopes nest: when one is already open, the context is returned unchanged and flush is a no-op,
// so an outer parse still reports everything exactly once. Call it at the outermost parse entry
// point and `defer flush(ctx)` there - flushing outside any enclosing process block, since the
// report is framed with its own separators.
func withDeprecationCollector(ctx context.Context) (context.Context, func(context.Context)) {
	if collectorFromContext(ctx) != nil {
		return ctx, func(context.Context) {}
	}

	c := &deprecationCollector{}
	return context.WithValue(ctx, deprecationCollectorKey{}, c), c.flush
}

func collectorFromContext(ctx context.Context) *deprecationCollector {
	c, _ := ctx.Value(deprecationCollectorKey{}).(*deprecationCollector)
	return c
}

func (c *deprecationCollector) add(kind, path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.seen == nil {
		c.seen = make(map[string]struct{})
		c.fields = make(map[string][]string)
	}

	key := kind + "\x00" + path
	if _, ok := c.seen[key]; ok {
		return
	}
	c.seen[key] = struct{}{}

	if _, ok := c.fields[kind]; !ok {
		c.order = append(c.order, kind)
	}
	c.fields[kind] = append(c.fields[kind], path)
}

// flush reports everything collected so far and empties the collector, so a repeated flush (a
// deferred call on an error path that already reported) stays silent.
//
// Each option gets its own DEPRECATED milestone naming the document and the field. Milestones are
// curated output: the interactive UI keeps them on screen after the live block is torn down, so a
// deprecation found while the configuration is parsed is still visible when the operation ends
// twenty minutes later, instead of having scrolled away with the rest of the detail. The prose -
// what being deprecated means and what to do about it - is said once, in a single warning after
// the list, rather than repeated around every field.
func (c *deprecationCollector) flush(ctx context.Context) {
	c.mu.Lock()
	order, fields := c.order, c.fields
	c.order, c.fields, c.seen = nil, nil, nil
	c.mu.Unlock()

	if len(order) == 0 {
		return
	}

	logger := dhlog.FromContext(ctx)

	total := 0
	for _, kind := range order {
		// Schema properties are walked in map order, so sort the paths to keep the report stable
		// between runs of the same configuration.
		sort.Strings(fields[kind])
		total += len(fields[kind])

		for _, path := range fields[kind] {
			logger.InfoContext(ctx, kind+": "+path, dhlog.ShowInCompacted(), dhlog.BadgeDeprecated())
		}
	}

	logger.WarnContext(ctx, deprecationNotice(total))
}

// deprecationNotice phrases the explanation that follows the list. Singular and plural are written
// out rather than assembled from fragments: the sentence needs "it"/"them" and "is"/"are" to agree,
// and a half-built sentence reads worse than two full ones.
func deprecationNotice(total int) string {
	if total == 1 {
		return "1 deprecated option is set in the configuration, listed above. " +
			"Support for it will be removed in a future release - update the configuration before upgrading."
	}
	return fmt.Sprintf(
		"%d deprecated options are set in the configuration, listed above. "+
			"Support for them will be removed in a future release - update the configuration before upgrading.",
		total)
}

// kindLabel renders "Kind" or, when the document has a metadata.name, `Kind "name"`.
func kindLabel(index *SchemaIndex, name string) string {
	if name == "" {
		return index.Kind
	}
	return fmt.Sprintf("%s %q", index.Kind, name)
}
