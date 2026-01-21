package cluster

type State struct {
	Spec
	Status
}

type Spec struct {
	DesiredVersion string     `yaml:"desiredVersion"`
	UpdateMode     UpdateMode `yaml:"updateMode"`
}

type Status struct {
	ControlPlaneState ControlPlaneState `json:"controlPlane" yaml:"controlPlane"`
	NodesState        NodesState        `json:"nodes" yaml:"nodes"`
	Phase             Phase             `json:"phase" yaml:"phase"`
}

type Phase string

const (
	ClusterControlPlaneUpdating     Phase = "ControlPlaneUpdating"
	ClusterControlPlaneVersionDrift Phase = "ControlPlaneVersionDrift"
	ClusterControlPlaneInconsistent Phase = "ControlPlaneInconsistent"
	ClusterUpToDate                 Phase = "UpToDate"
	ClusterNodesUpdating            Phase = "NodesUpdating"
	ClusterVersionDrift             Phase = "VersionDrift"
	ClusterInconsistent             Phase = "Inconsistent"
	ClusterUnknown                  Phase = "Unknown"
)
