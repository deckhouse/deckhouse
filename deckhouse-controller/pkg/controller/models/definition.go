package models

const (
	ModuleDefinitionFile = "module.yaml"
)

type DeckhouseModuleDefinition struct {
	Name        string   `yaml:"name"`
	Weight      uint32   `yaml:"weight"`
	Tags        []string `yaml:"tags"`
	Description string   `yaml:"description"`

	Path string `yaml:"-"`
}
