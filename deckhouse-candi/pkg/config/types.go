package config

type SchemaIndex struct {
	Kind    string `json:"kind"`
	Version string `json:"apiVersion"`
}

func (i *SchemaIndex) IsValid() bool {
	return i.Kind != "" && i.Version != ""
}

type OpenAPISchema struct {
	Kind     string                 `json:"kind"`
	Versions []OpenAPISchemaVersion `json:"apiVersions"`
}

type OpenAPISchemaVersion struct {
	Version string      `json:"apiVersion"`
	Schema  interface{} `json:"openAPISpec"`
}

type ClusterConfigSpec struct {
	ClusterType       string                 `json:"clusterType"`
	Cloud             map[string]interface{} `json:"cloud"`
	KubernetesVersion string                 `json:"kubernetesVersion"`
	ServiceSubnetCIDR string                 `json:"serviceSubnetCIDR"`
	ClusterDomain     string                 `json:"clusterDomain"`
}

type ProviderClusterConfigSpec struct {
	Layout string `json:"layout"`
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

type NodeGroup struct {
	Kind       string        `json:"kind"`
	APIVersion string        `json:"apiVersion"`
	Spec       NodeGroupSpec `json:"spec"`
}

type NodeGroupSpec struct {
	NodeType       string                 `json:"nodeType"`
	CloudInstances map[string]interface{} `json:"cloudInstances"`
	Bashible       interface{}            `json:"bashible"`
}

type ClassReference struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}
