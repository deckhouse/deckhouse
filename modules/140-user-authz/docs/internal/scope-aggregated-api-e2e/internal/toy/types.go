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

// Package toy is a minimal aggregated API server for trying the scope receiver
// library: group scopetoy.deckhouse.io/v1, namespaced resource gizmos, kept in
// memory.
package toy

import (
	extopenapi "k8s.io/apiextensions-apiserver/pkg/generated/openapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

const GroupName = "scopetoy.deckhouse.io"

var (
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1"}
	GizmoResource      = schema.GroupResource{Group: GroupName, Resource: "gizmos"}
)

// Gizmo is the toy object.
type Gizmo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              GizmoSpec `json:"spec,omitempty"`
}

type GizmoSpec struct {
	Color string `json:"color,omitempty"`
}

type GizmoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Gizmo `json:"items"`
}

// Hand-written deep copies: codegen is avoided on purpose.

func (in *Gizmo) DeepCopyInto(out *Gizmo) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
}

func (in *Gizmo) DeepCopy() *Gizmo {
	if in == nil {
		return nil
	}
	out := new(Gizmo)
	in.DeepCopyInto(out)
	return out
}

func (in *Gizmo) DeepCopyObject() runtime.Object { return in.DeepCopy() }

func (in *GizmoList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(GizmoList)
	*out = *in
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]Gizmo, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
	return out
}

// OpenAPI model names. The generic server needs OpenAPI v3 definitions of
// every served type to build the server-side-apply type converter.
func (Gizmo) OpenAPIModelName() string     { return "io.deckhouse.scopetoy.v1.Gizmo" }
func (GizmoSpec) OpenAPIModelName() string { return "io.deckhouse.scopetoy.v1.GizmoSpec" }
func (GizmoList) OpenAPIModelName() string { return "io.deckhouse.scopetoy.v1.GizmoList" }

var (
	Scheme = runtime.NewScheme()
	Codecs = serializer.NewCodecFactory(Scheme)

	ParameterCodec = runtime.NewParameterCodec(Scheme)
)

func init() {
	// The same Go types serve as the internal version: conversion between
	// the two is the identity.
	for _, gv := range []schema.GroupVersion{SchemeGroupVersion, {Group: GroupName, Version: runtime.APIVersionInternal}} {
		Scheme.AddKnownTypes(gv, &Gizmo{}, &GizmoList{})
	}
	metav1.AddToGroupVersion(Scheme, SchemeGroupVersion)
	if err := Scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("Gizmo"), func(label, value string) (string, string, error) {
		switch label {
		case "metadata.name", "metadata.namespace":
			return label, value, nil
		}
		return "", "", errUnsupportedField(label)
	}); err != nil {
		panic(err)
	}

	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})
	Scheme.AddUnversionedTypes(schema.GroupVersion{Group: "", Version: "v1"},
		&metav1.Status{}, &metav1.APIVersions{}, &metav1.APIGroupList{}, &metav1.APIGroup{}, &metav1.APIResourceList{})
}

type errUnsupportedField string

func (e errUnsupportedField) Error() string { return "field label not supported: " + string(e) }

// GetOpenAPIDefinitions returns the definitions of the toy types plus the
// generated ones of meta/v1 (borrowed from apiextensions-apiserver).
func GetOpenAPIDefinitions(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
	defs := extopenapi.GetOpenAPIDefinitions(ref)
	objectMeta := metav1.ObjectMeta{}.OpenAPIModelName()
	listMeta := metav1.ListMeta{}.OpenAPIModelName()
	str := func(desc string) spec.Schema {
		return spec.Schema{SchemaProps: spec.SchemaProps{Type: []string{"string"}, Description: desc}}
	}
	defs[GizmoSpec{}.OpenAPIModelName()] = common.OpenAPIDefinition{
		Schema: spec.Schema{SchemaProps: spec.SchemaProps{
			Type:       []string{"object"},
			Properties: map[string]spec.Schema{"color": str("Color of the gizmo.")},
		}},
	}
	defs[Gizmo{}.OpenAPIModelName()] = common.OpenAPIDefinition{
		Schema: spec.Schema{SchemaProps: spec.SchemaProps{
			Type: []string{"object"},
			Properties: map[string]spec.Schema{
				"apiVersion": str("APIVersion of the object."),
				"kind":       str("Kind of the object."),
				"metadata":   {SchemaProps: spec.SchemaProps{Default: map[string]any{}, Ref: ref(objectMeta)}},
				"spec":       {SchemaProps: spec.SchemaProps{Default: map[string]any{}, Ref: ref(GizmoSpec{}.OpenAPIModelName())}},
			},
		}},
		Dependencies: []string{objectMeta, GizmoSpec{}.OpenAPIModelName()},
	}
	defs[GizmoList{}.OpenAPIModelName()] = common.OpenAPIDefinition{
		Schema: spec.Schema{SchemaProps: spec.SchemaProps{
			Type:     []string{"object"},
			Required: []string{"items"},
			Properties: map[string]spec.Schema{
				"apiVersion": str("APIVersion of the list."),
				"kind":       str("Kind of the list."),
				"metadata":   {SchemaProps: spec.SchemaProps{Default: map[string]any{}, Ref: ref(listMeta)}},
				"items": {SchemaProps: spec.SchemaProps{
					Type:  []string{"array"},
					Items: &spec.SchemaOrArray{Schema: &spec.Schema{SchemaProps: spec.SchemaProps{Default: map[string]any{}, Ref: ref(Gizmo{}.OpenAPIModelName())}}},
				}},
			},
		}},
		Dependencies: []string{listMeta, Gizmo{}.OpenAPIModelName()},
	}
	return defs
}
