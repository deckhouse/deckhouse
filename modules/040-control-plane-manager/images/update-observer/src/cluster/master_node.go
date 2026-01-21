package cluster

type MasterNodeState struct {
	Phase           MasterNodePhase             `json:"phase" yaml:"phase"`
	ComponentsState ControlPlaneComponentsState `json:",inline" yaml:",inline"`
}

type MasterNodePhase string

const (
	MasterNodeUptoDate MasterNodePhase = "UpToDate"
	MasterNodeUpdating MasterNodePhase = "Updating"
)

func (n *MasterNodeState) isUpToDate() bool {
	return n.Phase == MasterNodeUptoDate
}
