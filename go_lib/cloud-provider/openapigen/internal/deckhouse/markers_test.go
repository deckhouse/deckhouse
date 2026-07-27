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

package deckhouse

import (
	"errors"
	"reflect"

	"github.com/getkin/kin-openapi/openapi3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-tools/pkg/markers"

	pkgmarkers "openapigen/markers"
)

type testSchemaMarker struct {
	value string
}

func (m testSchemaMarker) ApplyToSchema(schema *openapi3.Schema) error {
	schema.Title = m.value
	return nil
}

type notASchemaMarker struct{}

type customType struct {
	Name string `json:"name,omitempty"`
}

// testMergeableMarker is a fake marker used to verify normalizeMarkerValues.
// It implements pkgmarkers.MergeableSchemaMarker: collapses payloads into one marker.
type testMergeableMarker struct {
	payload string
}

func (m testMergeableMarker) ApplyToSchema(schema *openapi3.Schema) error {
	schema.Title = m.payload
	return nil
}

func (m testMergeableMarker) MergeFrom(occurrences []any) (pkgmarkers.SchemaMarker, error) {
	var combined string
	for _, raw := range occurrences {
		t, ok := raw.(testMergeableMarker)
		if !ok {
			return nil, errors.New("foreign type")
		}
		combined += t.payload
	}
	return testMergeableMarker{payload: combined}, nil
}

// failingMergeableMarker always returns an error from MergeFrom — used to verify error wrapping.
type failingMergeableMarker struct{}

func (m failingMergeableMarker) ApplyToSchema(schema *openapi3.Schema) error {
	return nil
}

func (m failingMergeableMarker) MergeFrom(occurrences []any) (pkgmarkers.SchemaMarker, error) {
	return nil, errors.New("boom")
}

var _ = Describe("Processor markers", func() {
	Describe("applyMarkerValuesToSchema", func() {
		It("applies schemaMarker values to schema", func() {
			schema := &openapi3.Schema{}
			mv := markers.MarkerValues{
				"deckhouse:title": []any{testSchemaMarker{value: "my-title"}},
			}

			err := applyMarkerValuesToSchema(schema, mv)
			Expect(err).NotTo(HaveOccurred())
			Expect(schema.Title).To(Equal("my-title"))
		})

		It("returns error for marker values without schemaMarker interface", func() {
			schema := &openapi3.Schema{}
			mv := markers.MarkerValues{
				"deckhouse:title": []any{notASchemaMarker{}},
			}

			err := applyMarkerValuesToSchema(schema, mv)
			Expect(err).To(MatchError(ContainSubstring("does not implement schemaMarker interface")))
		})
	})

	Describe("normalizeMarkerValues", func() {
		It("merges occurrences of MergeableSchemaMarker into one", func() {
			mv := markers.MarkerValues{
				"x": []any{
					testMergeableMarker{payload: "a"},
					testMergeableMarker{payload: "b"},
					testMergeableMarker{payload: "c"},
				},
			}

			err := normalizeMarkerValues(mv)
			Expect(err).NotTo(HaveOccurred())
			Expect(mv["x"]).To(HaveLen(1))

			merged, ok := mv["x"][0].(testMergeableMarker)
			Expect(ok).To(BeTrue())
			Expect(merged.payload).To(Equal("abc"))
		})

		It("leaves plain SchemaMarker values untouched", func() {
			mv := markers.MarkerValues{
				"deckhouse:title": []any{
					testSchemaMarker{value: "one"},
					testSchemaMarker{value: "two"},
				},
			}

			err := normalizeMarkerValues(mv)
			Expect(err).NotTo(HaveOccurred())
			Expect(mv["deckhouse:title"]).To(HaveLen(2))
			Expect(mv["deckhouse:title"][0]).To(Equal(testSchemaMarker{value: "one"}))
			Expect(mv["deckhouse:title"][1]).To(Equal(testSchemaMarker{value: "two"}))
		})

		It("wraps MergeFrom errors with marker name", func() {
			mv := markers.MarkerValues{
				"my-marker": []any{failingMergeableMarker{}},
			}

			err := normalizeMarkerValues(mv)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("my-marker"))
			Expect(err.Error()).To(ContainSubstring("boom"))
		})
	})

	Describe("jsonNameFromTag", func() {
		It("extracts field name from json tag", func() {
			name, ok := jsonNameFromTag(reflect.StructTag(`json:"name,omitempty"`))
			Expect(ok).To(BeTrue())
			Expect(name).To(Equal("name"))
		})

		It("returns false for missing json tag", func() {
			name, ok := jsonNameFromTag(reflect.StructTag(`yaml:"name"`))
			Expect(ok).To(BeFalse())
			Expect(name).To(BeEmpty())
		})
	})

	Describe("normalizeStructType", func() {
		It("unwraps pointer chain to struct type", func() {
			t := normalizeStructType(reflect.TypeOf(new(*customType)))
			Expect(t.Kind()).To(Equal(reflect.Struct))
			Expect(t.Name()).To(Equal("customType"))
		})
	})
})
