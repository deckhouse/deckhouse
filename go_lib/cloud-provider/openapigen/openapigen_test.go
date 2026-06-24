package openapigen

import (
	"os"
	"path/filepath"
	"testing"

	crdmodelv1alpha1 "openapigen/internal/test/crdmodel/v1alpha1"
	"openapigen/internal/test/instanceclass"
	"openapigen/internal/test/module"
	multiv1 "openapigen/internal/test/multiversioncrd/v1"
	multiv1alpha1 "openapigen/internal/test/multiversioncrd/v1alpha1"
	"openapigen/internal/test/usermodel"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"sigs.k8s.io/yaml"
)

func TestOpenapigen(t *testing.T) {
	format.MaxLength = 30000
	RegisterFailHandler(Fail)
	RunSpecs(t, "OpenAPIGen Suite")
}

var testModelsPath = filepath.Join(
	"internal",
	"test",
)

var userOpenAPISchemaGoldenPath = filepath.Join(
	testModelsPath,
	"usermodel",
	"user_schema.golden.yaml",
)

var userRuDocGoldenPath = filepath.Join(
	testModelsPath,
	"usermodel",
	"user_schema_description_ru.golden.yaml",
)

var instanceClassOpenAPISchemaGoldenPath = filepath.Join(
	testModelsPath,
	"instanceclass",
	"instance_class_schema.golden.yaml",
)

var moduleConfigOpenAPISchemaGoldenPath = filepath.Join(
	testModelsPath,
	"module",
	"module_config_schema.golden.yaml",
)

var moduleConfigRuDocGoldenPath = filepath.Join(
	testModelsPath,
	"module",
	"module_config_ru_doc.golden.yaml",
)

var testResourceCRDGoldenPath = filepath.Join(
	testModelsPath,
	"crdmodel",
	"v1alpha1",
	"test_resource_crd.golden.yaml",
)

var multiVersionCRDGoldenPath = filepath.Join(
	testModelsPath,
	"multiversioncrd",
	"multiversion_crd.golden.yaml",
)

var _ = Describe("SchemaGenerator", func() {
	Describe("NewSchemaGenerator", func() {
		It("returns error when both flags are false", func() {
			gen, err := NewSchemaGenerator(SchemaConfig{})
			Expect(err).To(HaveOccurred())
			Expect(gen).To(BeNil())
		})
	})

	Describe("Generate", func() {
		It("is idempotent: same root called twice returns identical results", func() {
			gen, err := NewSchemaGenerator(SchemaConfig{
				EnableKubebuilderMarkers: true,
				EnableDeckhouseMarkers:   true,
			})
			Expect(err).NotTo(HaveOccurred())

			first, err := gen.Generate(usermodel.User{})
			Expect(err).NotTo(HaveOccurred())

			second, err := gen.Generate(usermodel.User{})
			Expect(err).NotTo(HaveOccurred())

			firstYAML, err := yaml.Marshal(first)
			Expect(err).NotTo(HaveOccurred())
			secondYAML, err := yaml.Marshal(second)
			Expect(err).NotTo(HaveOccurred())

			Expect(firstYAML).To(MatchYAML(secondYAML))
		})

		It("returns different results for different roots (no state leak)", func() {
			gen, err := NewSchemaGenerator(SchemaConfig{
				EnableKubebuilderMarkers: true,
				EnableDeckhouseMarkers:   true,
			})
			Expect(err).NotTo(HaveOccurred())

			userSchema, err := gen.Generate(usermodel.User{})
			Expect(err).NotTo(HaveOccurred())

			moduleSchema, err := gen.Generate(module.ModuleConfigSettings{})
			Expect(err).NotTo(HaveOccurred())

			userYAML, err := yaml.Marshal(userSchema)
			Expect(err).NotTo(HaveOccurred())
			moduleYAML, err := yaml.Marshal(moduleSchema)
			Expect(err).NotTo(HaveOccurred())

			Expect(userYAML).NotTo(MatchYAML(moduleYAML))
		})
	})
})

var _ = Describe("OpenAPIGen", func() {
	// Mergo regression: instanceclass and module golden files contain enum/required from
	// kubebuilder markers, so these tests verify kubebuilder+deckhouse schema merge is correct.
	Describe("GenerateDeckhouseOpenAPISchema", func() {
		It("user model renders correctly", func() {
			want, err := os.ReadFile(userOpenAPISchemaGoldenPath)
			Expect(err).To(BeNil())

			got, err := GenerateDeckhouseOpenAPISchema(usermodel.User{})
			Expect(err).To(BeNil())
			Expect(got).To(MatchYAML(want))
		})

		It("module config settings renders correctly", func() {
			want, err := os.ReadFile(moduleConfigOpenAPISchemaGoldenPath)
			Expect(err).To(BeNil())

			got, err := GenerateDeckhouseOpenAPISchema(module.ModuleConfigSettings{})
			Expect(err).To(BeNil())
			Expect(got).To(MatchYAML(want))
		})

		It("instance class renders correctly", func() {
			want, err := os.ReadFile(instanceClassOpenAPISchemaGoldenPath)
			Expect(err).To(BeNil())

			got, err := GenerateDeckhouseOpenAPISchema(instanceclass.InstanceClass{})
			Expect(err).To(BeNil())
			Expect(got).To(MatchYAML(want))
		})
	})

	Describe("GenerateDeckhouseDescriptionRu", func() {
		It("returns schema with ru description for user id field", func() {
			want, err := os.ReadFile(userRuDocGoldenPath)
			Expect(err).NotTo(HaveOccurred())

			got, err := GenerateDeckhouseDescriptionRu(usermodel.User{})
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(MatchYAML(want))
		})

		It("returns ru-description schema for module config", func() {
			want, err := os.ReadFile(moduleConfigRuDocGoldenPath)
			Expect(err).NotTo(HaveOccurred())

			got, err := GenerateDeckhouseDescriptionRu(module.ModuleConfigSettings{})
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(MatchYAML(want))
		})
	})

	Describe("GenerateCRD", func() {
		It("test resource CRD renders correctly", func() {
			got, err := GenerateCRD([]VersionSpec{{Root: &crdmodelv1alpha1.TestResource{}}})
			Expect(err).NotTo(HaveOccurred())

			if _, statErr := os.Stat(testResourceCRDGoldenPath); os.IsNotExist(statErr) {
				err = os.MkdirAll(filepath.Dir(testResourceCRDGoldenPath), 0755)
				Expect(err).NotTo(HaveOccurred())
				err = os.WriteFile(testResourceCRDGoldenPath, got, 0644)
				Expect(err).NotTo(HaveOccurred())
				return
			}

			want, err := os.ReadFile(testResourceCRDGoldenPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(MatchYAML(want))
		})

		It("multi-version CRD renders correctly", func() {
			got, err := GenerateCRD([]VersionSpec{
				{Root: &multiv1alpha1.MultiVersionResource{}},
				{Root: &multiv1.MultiVersionResource{}},
			})
			Expect(err).NotTo(HaveOccurred())

			if _, statErr := os.Stat(multiVersionCRDGoldenPath); os.IsNotExist(statErr) {
				err = os.MkdirAll(filepath.Dir(multiVersionCRDGoldenPath), 0755)
				Expect(err).NotTo(HaveOccurred())
				err = os.WriteFile(multiVersionCRDGoldenPath, got, 0644)
				Expect(err).NotTo(HaveOccurred())
				return
			}

			want, err := os.ReadFile(multiVersionCRDGoldenPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(MatchYAML(want))
		})
	})
})

var _ = Describe("CRDGenerator", func() {
	It("returns error for empty versions", func() {
		gen, err := NewCRDGenerator(SchemaConfig{EnableKubebuilderMarkers: true, EnableDeckhouseMarkers: true})
		Expect(err).NotTo(HaveOccurred())
		_, err = gen.Generate(CRDMeta{}, []VersionSpec{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("at least one VersionSpec"))
	})

	It("returns error for nil Root", func() {
		gen, err := NewCRDGenerator(SchemaConfig{EnableKubebuilderMarkers: true, EnableDeckhouseMarkers: true})
		Expect(err).NotTo(HaveOccurred())
		_, err = gen.Generate(CRDMeta{}, []VersionSpec{{Root: nil}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Root must not be nil"))
	})

	It("returns error when EnableKubebuilderMarkers is false", func() {
		gen, err := NewCRDGenerator(SchemaConfig{EnableDeckhouseMarkers: true})
		Expect(err).NotTo(HaveOccurred())
		_, err = gen.Generate(CRDMeta{}, []VersionSpec{{Root: &crdmodelv1alpha1.TestResource{}}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("EnableKubebuilderMarkers"))
	})

	It("returns error for type without TypeMeta/ObjectMeta", func() {
		type bareStruct struct {
			Name string `json:"name"`
		}
		gen, err := NewCRDGenerator(SchemaConfig{EnableKubebuilderMarkers: true, EnableDeckhouseMarkers: true})
		Expect(err).NotTo(HaveOccurred())
		_, err = gen.Generate(CRDMeta{}, []VersionSpec{{Root: &bareStruct{}}})
		Expect(err).To(HaveOccurred())
	})
})
