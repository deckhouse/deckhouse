package nodecondition

import (
	"fmt"
	"os/exec"

	"d8_shutdown_inhibitor/pkg/kubernetes"
)

const (
	GracefulShutdownPostponeType = "GracefulShutdownPostpone"
	ReasonOnStart                = "ShutdownInhibitorIsStarted"
	ReasonOnUnlock               = "NoRunningPodsWithLabel"
	ReasonPodsArePresent         = "PodsWithLabelAreRunningOnNode"
)

func GracefulShutdownPostpone() *gracefulShutdownPostpone {
	return &gracefulShutdownPostpone{}
}

type gracefulShutdownPostpone struct{}

func (g *gracefulShutdownPostpone) SetOnStart(nodeName string) error {
	return patchGracefulShutdownPostponeCondition(nodeName, ReasonOnStart, StatusTrue)
}

func (g *gracefulShutdownPostpone) SetPodsArePresent(nodeName string) error {
	return patchGracefulShutdownPostponeCondition(nodeName, ReasonPodsArePresent, StatusTrue)
}

func (g *gracefulShutdownPostpone) UnsetOnUnlock(nodeName string) error {
	return patchGracefulShutdownPostponeCondition(nodeName, ReasonOnUnlock, StatusFalse)
}

// patchGracefulShutdownPostponeCondition updates GracefulShutdownPostpone condition.
/*
kubectl patch node/static-vm-node-00 --type strategic
-p '{"status":{"conditions":[{"type":"GracefulShutdownPostpone", "status":"True", "reason":"PodsWithLabelAreRunningOnNode"}]}}'
--subresource=status

kubectl patch node/static-vm-node-00 --type strategic
-p '{"status":{"conditions":[{"type":"GracefulShutdownPostpone", "status":"False", "reason":"NoRunningPodsWithLabel"}]}}'
--subresource=status
*/
func patchGracefulShutdownPostponeCondition(nodeName, status, reason string) error {
	k := kubernetes.NewDefaultKubectl()
	err := k.PatchCondition("Node", nodeName, GracefulShutdownPostponeType, status, reason, "")
	return reformatExitError(err)
}

func reformatExitError(err error) error {
	if err == nil {
		return nil
	}
	ee, ok := err.(*exec.ExitError)
	if ok && len(ee.Stderr) > 0 {
		return fmt.Errorf("%v: %s", err, string(ee.Stderr))
	}
	return err
}
