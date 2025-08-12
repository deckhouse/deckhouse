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

	return patchGracefulShutdownPostponeCondition(nodeName, StatusFalse, ReasonOnStart)
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

func nodeIsReady(k *kubernetes.Kubectl, nodeName string) (bool, error) {
	nodeNotReadyCondition, err := k.GetCondition(nodeName, "KubeletNotReady")
	if err != nil {
		return false, reformatExitError(err)
	}
	if nodeNotReadyCondition != nil &&
		nodeNotReadyCondition.Status == "False" &&
		nodeNotReadyCondition.Type == "Ready" &&
		nodeNotReadyCondition.Reason == "KubeletNotReady" {
		return false, fmt.Errorf("node %q is not ready", nodeName)
	}
	return true, nil
}

func cordonedByInhibitor(k *kubernetes.Kubectl, nodeName string) (bool, error) {
	cordonBy, err := k.GetAnnotationCordonedBy(nodeName)
	if err != nil {
		fmt.Printf("uncordonOnStart: error getting cordonBy annotation: %v\n", err)
		return false, reformatExitError(err)
	}

	if cordonBy == kubernetes.CordonAnnotationValue {
		return true, nil
	}
	return false, nil
}

func uncordonAndCleanup(k *kubernetes.Kubectl, nodeName string) error {
	if _, err := k.Uncordon(nodeName); err != nil {
		fmt.Printf("uncordonAndCleanup: error during Uncordon: %v\n", err)
		return reformatExitError(err)
	}

	if _, err := k.RemoveCordonAnnotation(nodeName); err != nil {
		fmt.Printf("uncordonAndCleanup: error removing cordon annotation: %v\n", err)
		return reformatExitError(err)
	}
	return nil
}

func isShutdownInhibitedByPods(condition *kubernetes.Condition) bool {
	fmt.Printf("isShutdownInhibitedByPods: condition=%+v\n", condition)
	if condition == nil {
		return false
	}
	return condition.Status == "True" &&
		condition.Type == GracefulShutdownPostponeType &&
		condition.Reason == ReasonPodsArePresent
}

func uncordonOnStart(nodeName string) error {
	fmt.Printf("uncordonOnStart: start for node %q\n", nodeName)
	k := kubernetes.NewDefaultKubectl()

	// 1. isOurCordon?
	isOurCordon, err := cordonedByInhibitor(k, nodeName)
	if err != nil {
		return err
	}
	fmt.Printf("uncordonOnStart: isOurCordon %t\n", isOurCordon)

	if !isOurCordon {
		fmt.Println("uncordonOnStart: Node is not cordoned by inhibitor. No action needed")
		return nil
	}

	// 1. nodeIsReady?
	isReady, err := nodeIsReady(k, nodeName)
	if err != nil {
		isReady = false
	}
	fmt.Printf("uncordonOnStart: isReady %t\n", isReady)

	// 3. isInhibitorShutdownActive?
	podsPresentCondition, _ := k.GetCondition(nodeName, ReasonPodsArePresent)
	isInhibited := isShutdownInhibitedByPods(podsPresentCondition)
	fmt.Printf("uncordonOnStart: isInhibited %t\n", isInhibited)

	if !isReady && isInhibited {
		fmt.Println("uncordonOnStart: Node is NotReady and a valid shutdown signal is active. Holding cordon")
		return nil
	}
	if isReady {
		fmt.Println("uncordonOnStart: uncordonAndCleanup")
		// 4. Uncordon
		return uncordonAndCleanup(k, nodeName)
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
