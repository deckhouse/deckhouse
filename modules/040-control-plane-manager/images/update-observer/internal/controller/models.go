package controller

type ClusterConfiguration struct {
	KubernetesVersion string `yaml:"kubernetesVersion"`
	DesiredVersion    string `yaml:"desiredVersion"`
	UpdateMode        string
}

type NodesStatus struct {
	DesiredCount  int `json:"desiredCount" yaml:"desiredCount"`
	UpToDateCount int `json:"upToDateCount" yaml:"upToDateCount"`
}

type ControlPlaneStatus struct {
	DesiredCount   int    `json:"desiredCount" yaml:"desiredCount"`
	UpToDateCount  int    `json:"upToDateCount" yaml:"upToDateCount"`
	CurrentVersion string `json:"currentVersion" yaml:"currentVersion"`
	Progress       string `json:"progress" yaml:"progress"`
	State          string `json:"state" yaml:"state"`
}

type SpecData struct {
	DesiredVersion string `yaml:"desiredVersion"`
	UpdateMode     string `yaml:"updateMode"`
}

type Status struct {
	ControlPlane ControlPlaneStatus `json:"controlPlane" yaml:"controlPlane"`
	Nodes        NodesStatus        `json:"nodes" yaml:"nodes"`
	State        string             `json:"state" yaml:"state"`
}
