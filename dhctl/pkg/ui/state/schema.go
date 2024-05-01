package state

import (
	"github.com/go-openapi/spec"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

const (
	CloudCluster  = config.CloudClusterType
	StaticCluster = config.StaticClusterType
)

type Schema struct {
	store *config.SchemaStore
}

func NewSchema(store *config.SchemaStore) *Schema {
	return &Schema{
		store: store,
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

func (s *Schema) getClusterSchema() *spec.Schema {
	return s.store.Get(&config.SchemaIndex{
		Kind:    "ClusterConfiguration",
		Version: "deckhouse.io/v1",
	})
}
