package _import

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

type (
	PhaseData struct {
		ScanResult  *ScanResult
		CheckResult *check.CheckResult
	}
	OnPhaseFunc = phases.OnPhaseFunc[PhaseData]
)

const (
	ScanPhase    phases.OperationPhase = "Scan"
	CapturePhase phases.OperationPhase = "Capture"
	CheckPhase   phases.OperationPhase = "Check"
)
