/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package nodecondition

import (
	"context"
	"fmt"

	"d8_shutdown_inhibitor/pkg/kubernetes"

	v1 "k8s.io/api/core/v1"
)

const (
	GracefulShutdownPostponeType = "GracefulShutdownPostpone"
	ReasonOnStart                = "ShutdownInhibitorIsStarted"
	ReasonOnUnlock               = "NoRunningPodsWithLabel"
	ReasonPodsArePresent         = "PodsWithLabelAreRunningOnNode"
	ReasonPendindgState          = "Pending"
)

func GracefulShutdownPostpone(klient *kubernetes.Klient) *gracefulShutdownPostpone {
	return &gracefulShutdownPostpone{Klient: klient}
}

type gracefulShutdownPostpone struct {
	Klient *kubernetes.Klient
}

func (g *gracefulShutdownPostpone) SetOnStart(ctx context.Context, nodeName string) error {
	afterReboot, err := g.uncordonOnStart(ctx, nodeName)
	if err != nil {
		return err
	}
	if !afterReboot {
		return nil
	}
	return g.SetStatusUnknow(ctx, nodeName)
}

func (g *gracefulShutdownPostpone) SetStatusUnknow(ctx context.Context, nodeName string) error {
	return g.patchGracefulShutdownPostponeCondition(ctx, nodeName, StatusUnknown, ReasonPendindgState)
}

func (g *gracefulShutdownPostpone) SetPodsArePresent(ctx context.Context, nodeName string) error {
	return g.patchGracefulShutdownPostponeCondition(ctx, nodeName, StatusTrue, ReasonPodsArePresent)
}

func (g *gracefulShutdownPostpone) UnsetOnUnlock(ctx context.Context, nodeName string) error {
	return g.patchGracefulShutdownPostponeCondition(ctx, nodeName, StatusFalse, ReasonOnUnlock)
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
func (g *gracefulShutdownPostpone) patchGracefulShutdownPostponeCondition(ctx context.Context, nodeName, status, reason string) error {
	return g.Klient.GetNode(ctx, nodeName).
		PatchCondition(ctx, GracefulShutdownPostponeType, status, reason, "").
		Err()
}

func (g *gracefulShutdownPostpone) nodeShutdownInProgress(node *kubernetes.Node) (bool, error) {
	nodeNotReadyCondition, err := node.GetConditionByReason("KubeletNotReady")
	if err != nil {
		return false, err
	}
	return nodeNotReadyCondition.Status == v1.ConditionFalse &&
		nodeNotReadyCondition.Type == v1.NodeReady &&
		nodeNotReadyCondition.Message == "node is shutting down", nil
}

func (g *gracefulShutdownPostpone) cordonedByInhibitor(node *kubernetes.Node) (bool, error) {
	cordonBy, err := node.GetAnnotationCordonedBy()
	if err != nil {
		return false, fmt.Errorf("uncordonOnStart: error getting cordonBy annotation: %v", err)
	}
	return cordonBy == kubernetes.CordonAnnotationValue, nil
}

func (g *gracefulShutdownPostpone) uncordonAndCleanup(ctx context.Context, node *kubernetes.Node) error {
	if err := node.Uncordon(ctx).Err(); err != nil {
		return err
	}

	return g.Klient.GetNode(ctx, node.Name).RemoveCordonAnnotation(ctx).Err()
}

func (g *gracefulShutdownPostpone) isShutdownInhibitedByPods(condition v1.NodeCondition) bool {
	fmt.Printf("isShutdownInhibitedByPods: condition=%+v\n", condition)
	return condition.Status == "True" && condition.Type == GracefulShutdownPostponeType
}

func (g *gracefulShutdownPostpone) uncordonOnStart(ctx context.Context, nodeName string) (bool, error) {
	fmt.Printf("uncordonOnStart: start for node %q\n", nodeName)

	node := g.Klient.GetNode(ctx, nodeName)
	if err := node.Err(); err != nil {
		return false, err
	}

	isShutdownInProgress, err := g.nodeShutdownInProgress(node)
	if err != nil {
		return false, err
	}
	fmt.Printf("uncordonOnStart: isShutdownInProgress %t\n", isShutdownInProgress)

	podsPresentCondition, err := node.GetConditionByReason(ReasonPodsArePresent)
	isInhibited := g.isShutdownInhibitedByPods(podsPresentCondition)
	fmt.Printf("uncordonOnStart: isInhibited %t\n", isInhibited)

	if isShutdownInProgress && isInhibited {
		fmt.Println("uncordonOnStart: Node is NotReady and a valid shutdown signal is active. Holding cordon")
		return false, nil
	}

	fmt.Println("uncordonOnStart: uncordonAndCleanup")
	isOurCordon, err := g.cordonedByInhibitor(node)
	fmt.Printf("uncordonOnStart: isOurCordon %t\n", isOurCordon)
	if err != nil {
		return false, err
	}

	if !isOurCordon {
		fmt.Println("uncordonOnStart: Node is not cordoned by inhibitor. No action needed")
		return true, nil
	}

	return true, g.uncordonAndCleanup(ctx, node)
}
