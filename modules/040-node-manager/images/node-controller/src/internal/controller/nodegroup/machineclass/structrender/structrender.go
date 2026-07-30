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

// Package structrender is the phase-2 proof of concept for the v2 provider machine-template
// contract: instead of an executable text/template, the provider ships a plain YAML skeleton
// plus declarative field bindings, and node-controller builds the object structurally. No
// text templating is involved, so a hostile or malformed value cannot change the object
// shape, and a binding referencing a missing required input fails loudly at render time.
package structrender

import (
	"fmt"
	"strconv"
	"strings"

	sigsyaml "sigs.k8s.io/yaml"
)

// Spec is the parsed v2 template: a static object skeleton plus bindings that write
// computed values into it.
type Spec struct {
	// Object is the static part of the rendered object (apiVersion, kind, constant fields).
	Object map[string]interface{} `json:"object"`
	// Bindings are applied in order; each writes one value at Path.
	Bindings []Binding `json:"bindings"`
}

type Binding struct {
	// Path is a dot-separated location in the object ("spec.template.spec.zoneID").
	Path  string `json:"path"`
	Value Value  `json:"value"`
}

// Value computes one value from the render context. Exactly one of Const / From / List /
// ForEach / Object is the producer; Default / Lookup / Format / Required post-process it.
type Value struct {
	// Const is a literal.
	Const interface{} `json:"const,omitempty"`
	// From reads a dot-path from the context (roots: instanceClass, cloudProvider, zone,
	// templateName, checksum, nodeGroupName, item — inside forEach).
	From string `json:"from,omitempty"`
	// Default is used when From resolved to nothing. Either a literal…
	Default interface{} `json:"default,omitempty"`
	// …or another Value (e.g. read a cloudProvider fallback).
	DefaultFrom *Value `json:"defaultFrom,omitempty"`
	// Lookup indexes the map produced so far with a computed key (zoneToSubnetIdMap[zone]).
	Lookup *Lookup `json:"lookup,omitempty"`
	// Format applies printf to the produced scalar; %d coerces the value to int first
	// (numbers arrive as float64 from JSON/YAML).
	Format string `json:"format,omitempty"`
	// Required makes an absent value a render error instead of an omitted field.
	Required bool `json:"required,omitempty"`
	// List builds a list from element specs; elements that resolve to nothing are dropped.
	List []Value `json:"list,omitempty"`
	// ForEach maps every element of a context list through Element (bound as "item").
	ForEach string `json:"forEach,omitempty"`
	Element *Value `json:"element,omitempty"`
	// Object builds a nested object; absent members are omitted.
	Object map[string]Value `json:"object,omitempty"`
}

type Lookup struct {
	Map string `json:"map"` // context dot-path to a map
	Key Value  `json:"key"` // computed key
}

// Context carries everything a v2 template may reference — the same inputs the v1
// text/template context exposes, minus the ability to run code on them.
type Context struct {
	InstanceClass map[string]interface{}
	CloudProvider map[string]interface{}
	Zone          string
	TemplateName  string
	Checksum      string
	NodeGroupName string
}

func ParseSpec(raw []byte) (*Spec, error) {
	s := &Spec{}
	if err := sigsyaml.UnmarshalStrict(raw, s); err != nil {
		return nil, fmt.Errorf("parse v2 template: %w", err)
	}
	if s.Object == nil {
		return nil, fmt.Errorf("parse v2 template: no object skeleton")
	}
	return s, nil
}

// Render deep-copies the skeleton and applies the bindings.
func Render(spec *Spec, rctx Context) (map[string]interface{}, error) {
	obj := deepCopyMap(spec.Object)
	ctx := map[string]interface{}{
		"instanceClass": rctx.InstanceClass,
		"cloudProvider": rctx.CloudProvider,
		"zone":          rctx.Zone,
		"templateName":  rctx.TemplateName,
		"checksum":      rctx.Checksum,
		"nodeGroupName": rctx.NodeGroupName,
	}
	for _, b := range spec.Bindings {
		v, ok, err := eval(&b.Value, ctx)
		if err != nil {
			return nil, fmt.Errorf("binding %s: %w", b.Path, err)
		}
		if !ok {
			continue // absent and not required → field omitted, like {{- if }} in v1
		}
		if err := setPath(obj, b.Path, v); err != nil {
			return nil, fmt.Errorf("binding %s: %w", b.Path, err)
		}
	}
	return obj, nil
}

// eval computes a Value. ok=false means "absent": the binding is skipped.
func eval(v *Value, ctx map[string]interface{}) (interface{}, bool, error) {
	out, ok, err := produce(v, ctx)
	if err != nil {
		return nil, false, err
	}
	if !ok && v.DefaultFrom != nil {
		out, ok, err = eval(v.DefaultFrom, ctx)
		if err != nil {
			return nil, false, err
		}
	}
	if !ok && v.Default != nil {
		out, ok = v.Default, true
	}
	if !ok {
		if v.Required {
			return nil, false, fmt.Errorf("required value is absent (from=%q)", v.From)
		}
		return nil, false, nil
	}
	if v.Lookup != nil {
		m, mok, err := resolvePath(ctx, v.Lookup.Map)
		if err != nil || !mok {
			return nil, false, fmt.Errorf("lookup map %q is absent", v.Lookup.Map)
		}
		asMap, isMap := m.(map[string]interface{})
		if !isMap {
			return nil, false, fmt.Errorf("lookup map %q is not a map", v.Lookup.Map)
		}
		key, kok, err := eval(&v.Lookup.Key, ctx)
		if err != nil || !kok {
			return nil, false, fmt.Errorf("lookup key is absent: %v", err)
		}
		out, ok = asMap[fmt.Sprintf("%v", key)]
		if !ok {
			if v.Required {
				return nil, false, fmt.Errorf("lookup %q has no key %v", v.Lookup.Map, key)
			}
			return nil, false, nil
		}
	}
	if v.Format != "" {
		out, err = formatScalar(v.Format, out)
		if err != nil {
			return nil, false, err
		}
	}
	return out, true, nil
}

func produce(v *Value, ctx map[string]interface{}) (interface{}, bool, error) {
	switch {
	case v.Const != nil:
		return v.Const, true, nil
	case v.From != "":
		got, ok, err := resolvePath(ctx, v.From)
		if err != nil {
			return nil, false, err
		}
		return got, ok, nil
	case v.List != nil:
		out := make([]interface{}, 0, len(v.List))
		for i := range v.List {
			item, ok, err := eval(&v.List[i], ctx)
			if err != nil {
				return nil, false, err
			}
			if !ok {
				continue
			}
			// A list element producing a list is spliced (covers "static head + forEach tail").
			if sub, isList := item.([]interface{}); isList {
				out = append(out, sub...)
				continue
			}
			out = append(out, item)
		}
		if len(out) == 0 {
			return nil, false, nil
		}
		return out, true, nil
	case v.ForEach != "":
		src, ok, err := resolvePath(ctx, v.ForEach)
		if err != nil || !ok {
			return nil, false, err
		}
		list, isList := src.([]interface{})
		if !isList {
			return nil, false, fmt.Errorf("forEach %q is not a list", v.ForEach)
		}
		if v.Element == nil {
			return list, len(list) > 0, nil
		}
		out := make([]interface{}, 0, len(list))
		for _, item := range list {
			itemCtx := make(map[string]interface{}, len(ctx)+1)
			for k, val := range ctx {
				itemCtx[k] = val
			}
			itemCtx["item"] = item
			got, ok, err := eval(v.Element, itemCtx)
			if err != nil {
				return nil, false, err
			}
			if ok {
				out = append(out, got)
			}
		}
		return out, len(out) > 0, nil
	case v.Object != nil:
		out := map[string]interface{}{}
		for k := range v.Object {
			member := v.Object[k]
			got, ok, err := eval(&member, ctx)
			if err != nil {
				return nil, false, err
			}
			if ok {
				out[k] = got
			}
		}
		return out, len(out) > 0, nil
	}
	if v.Lookup != nil {
		// Lookup-only value: the producer is the lookup itself.
		return nil, true, nil
	}
	return nil, false, fmt.Errorf("empty value spec")
}

// resolvePath walks a dot-path through nested maps. ok=false when any hop is missing,
// nil, or an empty string — matching how the v1 templates treat falsy values in {{ if }}.
func resolvePath(root map[string]interface{}, path string) (interface{}, bool, error) {
	var cur interface{} = root
	for _, part := range strings.Split(path, ".") {
		m, isMap := cur.(map[string]interface{})
		if !isMap {
			return nil, false, nil
		}
		next, ok := m[part]
		if !ok {
			return nil, false, nil
		}
		cur = next
	}
	if cur == nil {
		return nil, false, nil
	}
	if s, isStr := cur.(string); isStr && s == "" {
		return nil, false, nil
	}
	return cur, true, nil
}

func formatScalar(format string, v interface{}) (interface{}, error) {
	if strings.Contains(format, "%d") {
		n, err := toInt64(v)
		if err != nil {
			return nil, fmt.Errorf("format %q: %w", format, err)
		}
		return fmt.Sprintf(format, n), nil
	}
	return fmt.Sprintf(format, v), nil
}

func toInt64(v interface{}) (int64, error) {
	switch n := v.(type) {
	case float64:
		return int64(n), nil
	case int64:
		return n, nil
	case int:
		return int64(n), nil
	case string:
		return strconv.ParseInt(n, 10, 64)
	default:
		return 0, fmt.Errorf("not a number: %T", v)
	}
}

func setPath(obj map[string]interface{}, path string, value interface{}) error {
	parts := strings.Split(path, ".")
	cur := obj
	for _, p := range parts[:len(parts)-1] {
		next, ok := cur[p]
		if !ok {
			m := map[string]interface{}{}
			cur[p] = m
			cur = m
			continue
		}
		m, isMap := next.(map[string]interface{})
		if !isMap {
			return fmt.Errorf("path segment %q is not an object", p)
		}
		cur = m
	}
	cur[parts[len(parts)-1]] = value
	return nil
}

func deepCopyMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = deepCopyValue(v)
	}
	return out
}

func deepCopyValue(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		return deepCopyMap(t)
	case []interface{}:
		out := make([]interface{}, len(t))
		for i, e := range t {
			out[i] = deepCopyValue(e)
		}
		return out
	default:
		return v
	}
}
