package converge

import "github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"

type ConvergeStatus string

const (
	ConvergeStatusInSync                          ConvergeStatus = "InSync"
	ConvergeStatusConverged                       ConvergeStatus = "Converged"
	ConvergeStatusNeedApproveForDestructiveChange ConvergeStatus = "NeedApproveForDestructiveChange"
)

type ConvergeResult struct {
	Status      ConvergeStatus     `json:"status"`
	CheckResult *check.CheckResult `json:"check_result,omitempty"`
}
