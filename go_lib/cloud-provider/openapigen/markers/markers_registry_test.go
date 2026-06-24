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
