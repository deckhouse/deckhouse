package state

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-openapi/spec"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/validate"
)

const (
	CloudCluster  = config.CloudClusterType
	StaticCluster = config.StaticClusterType
)

const (
	RegistryHTTPS = "HTTPS"
	RegistryHTTP  = "HTTP"
)

type Schema struct {
	store   *config.SchemaStore
	edition string
}

func NewSchema(store *config.SchemaStore) *Schema {
	edRaw, err := os.ReadFile("/deckhouse/edition")
	if err != nil {
		log.DebugF("Failed to read edition from /deckhouse/edition: %s", err)
	}

	ed := strings.ToLower(strings.TrimSpace(string(edRaw)))
	if ed == "" {
		ed = "fe"
	}

	return &Schema{
		store:   store,
		edition: ed,
	}
}

func (s *Schema) CloudProviders() []string {
	cl := s.getClusterSchema()
	enum := cl.SchemaProps.Properties["cloud"].SchemaProps.Properties["provider"].SchemaProps.Enum
	res := make([]string, len(enum))
	for i := range enum {
		res[i] = enum[i].(string)
	}
	return res
}

func (s *Schema) K8sVersions() []string {
	cl := s.getClusterSchema()
	enum := cl.SchemaProps.Properties["kubernetesVersion"].SchemaProps.Enum
	res := make([]string, len(enum))
	for i := range enum {
		res[i] = enum[i].(string)
	}
	return res
}

func (s *Schema) ProviderSchema(p string) *spec.Schema {
	ss := s.store.Get(&config.SchemaIndex{
		Kind:    p + "ClusterConfiguration",
		Version: "deckhouse.io/v1",
	})

	pp := ss.SchemaProps.Properties["provider"]

	return &pp
}

func (s *Schema) getClusterSchema() *spec.Schema {
	return s.store.Get(&config.SchemaIndex{
		Kind:    "ClusterConfiguration",
		Version: "deckhouse.io/v1",
	})
}

func (s *Schema) ValidateImagesRepo(r string) error {
	schema := s.store.Get(&config.SchemaIndex{
		Kind:    "InitConfiguration",
		Version: "deckhouse.io/v1",
	})

	ss := schema.SchemaProps.Properties["deckhouse"].SchemaProps.Properties["imagesRepo"]

	_, err := validate.OpenAPIValidate(&ss, r)

	return err
}

func (s *Schema) DefaultRegistryRepo() string {
	if s.edition == "ce" {
		return ""
	}

	return fmt.Sprintf("registry.deckhouse.io/deckhouse/%s", s.edition)
}

func (s *Schema) DefaultRegistryUser() string {
	if s.edition == "ce" {
		return ""
	}

	return "license-token"
}

func (s *Schema) HasCreds() bool {
	return s.edition != "ce"
}
