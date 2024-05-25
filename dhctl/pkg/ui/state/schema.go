package state

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-openapi/spec"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/internal/utils"
)

const (
	CloudCluster  = config.CloudClusterType
	StaticCluster = config.StaticClusterType
)

const (
	RegistryHTTPS = "HTTPS"
	RegistryHTTP  = "HTTP"
)

const (
	FlannelHostGW = "HostGW"
	FlannelVxLAN  = "VXLAN"
)

const (
	CNIFlannel      = "Flannel"
	CNICilium       = "Cilium"
	CNISimpleBridge = "SimpleBridge"
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
	res := make([]string, 0, len(enum))
	for i := range enum {
		e := enum[i].(string)
		if _, err := s.ProviderSchema(e); err != nil {
			log.DebugF("Provider schema error: %v", err)
			continue
		}
		res = append(res, strings.TrimSpace(e))
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

func (s *Schema) ProviderSchema(p string) (*spec.Schema, error) {
	if p == "vSphere" {
		p = "Vsphere"
	}
	ss, err := s.store.GetOrError(&config.SchemaIndex{
		Kind:    p + "ClusterConfiguration",
		Version: "deckhouse.io/v1",
	})

	if err != nil {
		return nil, err
	}

	pp := ss.SchemaProps.Properties["provider"]
	return &pp, nil
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

	_, err := utils.OpenAPIValidate(&ss, r)

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

func (s *Schema) GetCNIsForProvider(provider string) []string {
	switch provider {
	// static
	case "":
		return []string{CNICilium, CNIFlannel}
	case "AWS":
		fallthrough
	case "GCP":
		fallthrough
	case "Yandex":
		fallthrough
	case "Azure":
		return []string{CNISimpleBridge, CNICilium}
	default:
		return []string{CNICilium}
	}
}

func (s *Schema) ReleaseChannels() []string {
	schema := s.store.ModuleConfigSchema("deckhouse")
	if schema == nil {
		panic("Cannot load module config deckhouse")
	}

	channels := schema.SchemaProps.Properties["releaseChannel"].SchemaProps.Enum
	res := make([]string, 0, len(channels))
	for i := range channels {
		res = append(res, channels[i].(string))
	}

	return res
}

func (s *Schema) validatePublicDomainTemplate(p string) error {
	schema := s.store.ModuleConfigSchema("global")
	if schema == nil {
		panic("Cannot load module config global")
	}

	ss := schema.SchemaProps.Properties["modules"].SchemaProps.Properties["publicDomainTemplate"]
	_, err := utils.OpenAPIValidate(&ss, p)

	return err
}

func (s *Schema) GetFlannelModes() []string {
	return []string{FlannelVxLAN, FlannelHostGW}
}
