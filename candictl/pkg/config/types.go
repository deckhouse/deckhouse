package config

import "fmt"

const (
	CloudClusterType  = "Cloud"
	StaticClusterType = "Static"
)

type SchemaIndex struct {
	Kind    string `json:"kind"`
	Version string `json:"apiVersion"`
}

func (i *SchemaIndex) IsValid() bool {
	return i.Kind != "" && i.Version != ""
}

func (i *SchemaIndex) String() string {
	return fmt.Sprintf("%s, %s", i.Kind, i.Version)
}

type OpenAPISchema struct {
	Kind     string                 `json:"kind"`
	Versions []OpenAPISchemaVersion `json:"apiVersions"`
}

type OpenAPISchemaVersion struct {
	Version string      `json:"apiVersion"`
	Schema  interface{} `json:"openAPISpec"`
}

type ClusterConfigCloudSpec struct {
	Provider string `json:"provider"`
	Prefix   string `json:"prefix"`
}

type MasterNodeGroupSpec struct {
	Replicas int `json:"replicas"`
}

type StaticNodeGroupSpec struct {
	Replicas     int                    `json:"replicas"`
	Name         string                 `json:"name"`
	NodeTemplate map[string]interface{} `json:"nodeTemplate"`
}

type DeckhouseClusterConfig struct {
	ReleaseChannel    string                 `json:"releaseChannel,omitempty"`
	DevBranch         string                 `json:"devBranch,omitempty"`
	Bundle            string                 `json:"bundle,omitempty"`
	LogLevel          string                 `json:"logLevel,omitempty"`
	ImagesRepo        string                 `json:"imagesRepo"`
	RegistryDockerCfg string                 `json:"registryDockerCfg,omitempty"`
	ConfigOverrides   map[string]interface{} `json:"configOverrides"`
}
