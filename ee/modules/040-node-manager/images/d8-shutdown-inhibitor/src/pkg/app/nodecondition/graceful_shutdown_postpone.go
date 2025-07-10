/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

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
	if err := uncordonOnStart(nodeName); err != nil {
		return err
	}

	return patchGracefulShutdownPostponeCondition(nodeName, StatusTrue, ReasonOnStart)
}

func (g *gracefulShutdownPostpone) SetPodsArePresent(nodeName string) error {
	return patchGracefulShutdownPostponeCondition(nodeName, StatusTrue, ReasonPodsArePresent)
}

func (g *gracefulShutdownPostpone) UnsetOnUnlock(nodeName string) error {
	return patchGracefulShutdownPostponeCondition(nodeName, StatusFalse, ReasonOnUnlock)
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

func uncordonOnStart(nodeName string) error {
	k := kubernetes.NewDefaultKubectl()

	condition, err := k.GetCondition(nodeName, ReasonPodsArePresent)
	if err != nil {
		return err
	}

	if condition.Status == "True" &&
		condition.Type == GracefulShutdownPostponeType &&
		condition.Reason == ReasonPodsArePresent {
		// inhibitor already in shutdown state
		return nil
	}

	cordonBy, err := k.GetAnnotationCordonedBy(nodeName)
	if err != nil {
		return reformatExitError(err)
	}

	if cordonBy == kubernetes.CordonAnnotationValue {
		// check that 'cordon' was set by inhibitor.
		if _, err := k.Uncordon(nodeName); err != nil {
			return reformatExitError(err)
		}
		if _, err := k.RemoveCordonAnnotation(nodeName); err != nil {
			return reformatExitError(err)
		}
	}

	return nil
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
