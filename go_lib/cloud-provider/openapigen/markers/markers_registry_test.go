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

package markers

import (
	"github.com/getkin/kin-openapi/openapi3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func objectSchema() *openapi3.Schema {
	s := &openapi3.Schema{}
	s.Type = &openapi3.Types{openapi3.TypeObject}
	return s
}

type foreignMarker struct{}

var _ = Describe("deckhouseDescriptionRuType", func() {
	It("MergeFrom concatenates Value of each occurrence", func() {
		m := deckhouseDescriptionRuType{}
		merged, err := m.MergeFrom([]any{
			deckhouseDescriptionRuType{Value: "a"},
			deckhouseDescriptionRuType{Value: "b"},
		})
		Expect(err).NotTo(HaveOccurred())

		typed, ok := merged.(deckhouseDescriptionRuType)
		Expect(ok).To(BeTrue())
		// appendString prepends the previous accumulator and appends "\n":
		//   appendString("",  "a") -> "a\n"
		//   appendString("a\n", "b") -> "a\nb\n"
		Expect(typed.Value).To(Equal("a\nb\n"))
	})

	It("MergeFrom returns error for empty occurrences", func() {
		m := deckhouseDescriptionRuType{}
		_, err := m.MergeFrom(nil)
		Expect(err).To(MatchError(ContainSubstring("empty occurrences")))
	})

	It("MergeFrom returns error for foreign type", func() {
		m := deckhouseDescriptionRuType{}
		_, err := m.MergeFrom([]any{
			deckhouseDescriptionRuType{Value: "a"},
			foreignMarker{},
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("foreignMarker"))
	})

	It("ApplyToSchema assigns Description (does not append)", func() {
		schema := &openapi3.Schema{Description: "old"}
		m := deckhouseDescriptionRuType{Value: "new"}
		Expect(m.ApplyToSchema(schema)).To(Succeed())
		Expect(schema.Description).To(Equal("new"))
	})
})

var _ = Describe("deckhouseValidationAdditionalPropertiesItemsPatternType", func() {
	It("returns error when schema type is not object", func() {
		schema := openapi3.NewStringSchema()
		m := deckhouseValidationAdditionalPropertiesItemsPatternType{Value: "pat"}
		err := m.ApplyToSchema(schema)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("can only be applied to types or maps"))
	})

	It("returns error when AdditionalProperties.Schema is nil", func() {
		schema := objectSchema()
		m := deckhouseValidationAdditionalPropertiesItemsPatternType{Value: "pat"}
		err := m.ApplyToSchema(schema)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("map[string][]string"))
	})

	It("sets Pattern on AdditionalProperties.Schema.Value.Items.Value", func() {
		schema := objectSchema()
		itemSchema := openapi3.NewStringSchema()
		arrSchema := openapi3.NewArraySchema()
		arrSchema.Items = openapi3.NewSchemaRef("", itemSchema)
		schema.AdditionalProperties.Schema = openapi3.NewSchemaRef("", arrSchema)

		m := deckhouseValidationAdditionalPropertiesItemsPatternType{Value: "somepattern"}
		Expect(m.ApplyToSchema(schema)).To(Succeed())
		Expect(schema.AdditionalProperties.Schema.Value.Items.Value.Pattern).To(Equal("somepattern"))
	})
})

var _ = Describe("deckhouseDisableAdditionalPropertiesType", func() {
	It("sets Has to false (disables additionalProperties) when Value is true", func() {
		schema := &openapi3.Schema{}
		m := deckhouseDisableAdditionalPropertiesType{Value: true}
		Expect(m.ApplyToSchema(schema)).To(Succeed())
		Expect(schema.AdditionalProperties.Has).NotTo(BeNil())
		Expect(*schema.AdditionalProperties.Has).To(BeFalse())
	})

	It("sets Has to true (enables additionalProperties) when Value is false", func() {
		schema := &openapi3.Schema{}
		m := deckhouseDisableAdditionalPropertiesType{Value: false}
		Expect(m.ApplyToSchema(schema)).To(Succeed())
		Expect(schema.AdditionalProperties.Has).NotTo(BeNil())
		Expect(*schema.AdditionalProperties.Has).To(BeTrue())
	})
})

var _ = Describe("deckhouseXDocExamplesType", func() {
	It("MergeFrom collects the value of every occurrence in order", func() {
		merged, err := deckhouseXDocExamplesType{}.MergeFrom([]any{
			deckhouseXDocExamplesType{Value: "first"},
			deckhouseXDocExamplesType{Value: 2},
			deckhouseXDocExamplesType{Value: map[string]any{"key": "value"}},
		})
		Expect(err).NotTo(HaveOccurred())

		schema := &openapi3.Schema{}
		Expect(merged.ApplyToSchema(schema)).To(Succeed())
		Expect(schema.Extensions[XDocExamplesExtensionKey]).To(Equal([]any{
			"first",
			2,
			map[string]any{"key": "value"},
		}))
	})

	It("MergeFrom returns error for empty occurrences", func() {
		_, err := deckhouseXDocExamplesType{}.MergeFrom(nil)
		Expect(err).To(HaveOccurred())
	})

	It("MergeFrom returns error for foreign type", func() {
		_, err := deckhouseXDocExamplesType{}.MergeFrom([]any{deckhouseXDocDefaultType{Value: "x"}})
		Expect(err).To(HaveOccurred())
	})

	It("ApplyToSchema wraps a single unmerged occurrence instead of writing null", func() {
		schema := &openapi3.Schema{}
		Expect(deckhouseXDocExamplesType{Value: "only"}.ApplyToSchema(schema)).To(Succeed())
		Expect(schema.Extensions[XDocExamplesExtensionKey]).To(Equal([]any{"only"}))
	})

	It("ApplyToSchema writes nothing when there is no value at all", func() {
		schema := &openapi3.Schema{}
		Expect(deckhouseXDocExamplesType{}.ApplyToSchema(schema)).To(Succeed())
		Expect(schema.Extensions).NotTo(HaveKey(XDocExamplesExtensionKey))
	})
})

var _ = Describe("deckhouseXDocDefaultType", func() {
	It("initializes a nil extension map", func() {
		schema := &openapi3.Schema{}
		Expect(deckhouseXDocDefaultType{Value: 42}.ApplyToSchema(schema)).To(Succeed())
		Expect(schema.Extensions[XDocDefaultExtensionKey]).To(Equal(42))
	})

	It("keeps extensions written by earlier markers", func() {
		schema := &openapi3.Schema{}
		Expect(deckhouseXDocSkipType{Value: true}.ApplyToSchema(schema)).To(Succeed())
		Expect(deckhouseXDocDefaultType{Value: "d"}.ApplyToSchema(schema)).To(Succeed())
		Expect(schema.Extensions[XDocSkipExtensionKey]).To(Equal(true))
		Expect(schema.Extensions[XDocDefaultExtensionKey]).To(Equal("d"))
	})
})
