/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const KubectlPath = "/opt/deckhouse/bin/kubectl"
const KubeConfigPath = "/etc/kubernetes/kubelet.conf"
const CordonAnnotationKey = "node.deckhouse.io/cordoned-by"
const CordonAnnotationValue = "shutdown-inhibitor"

type Kubectl struct {
	kubectlPath    string
	kubeConfigPath string
}

func NewDefaultKubectl() *Kubectl {
	return NewKubectlWithConf(KubectlPath, KubeConfigPath)
}

func NewKubectlWithConf(kubectlPath, kubeConfigPath string) *Kubectl {
	return &Kubectl{
		kubeConfigPath: kubeConfigPath,
		kubectlPath:    kubectlPath,
	}
}

func (k *Kubectl) Cordon(nodeName string) ([]byte, error) {
	cmd, cancel := k.cmd("cordon", nodeName)
	defer cancel()
	return cmd.CombinedOutput()
}

func (k *Kubectl) Uncordon(nodeName string) ([]byte, error) {
	cmd, cancel := k.cmd("uncordon", nodeName)
	defer cancel()
	return cmd.CombinedOutput()
}

func (k *Kubectl) ListPods(nodeName string) (*PodList, error) {
	out, err := k.listPods(nodeName)
	if err != nil {
		return nil, err
	}

	podList, err := podsListFromJSON(out)
	if err != nil {
		return nil, err
	}

	return podList, nil
}

func (k *Kubectl) GetAnnotationCordonedBy(nodeName string) (string, error) {
	out, err := k.getAnnotationCordonedBy(nodeName)
	if err != nil {
		return "", err
	}
	annotationStr := fmt.Sprintf("%s", bytes.Trim(out, `"'`))

	return annotationStr, nil
}

func (k *Kubectl) getAnnotationCordonedBy(nodeName string) ([]byte, error) {
	jsonPath := fmt.Sprintf("jsonpath='{.metadata.annotations.%s}'", strings.ReplaceAll(CordonAnnotationKey, ".", `\.`))
	cmd, cancel := k.cmd("get", "node", nodeName, "-o", jsonPath)
	defer cancel()
	return cmd.Output()
}

func (k *Kubectl) listPods(nodeName string) ([]byte, error) {
	nodeNameFieldSelector := fmt.Sprintf("spec.nodeName=%s", nodeName)
	cmd, cancel := k.cmd("get", "po", "-A", "-o", "json", "--field-selector", nodeNameFieldSelector)
	defer cancel()
	return cmd.Output()
}

func (k *Kubectl) PatchCondition(kind, name, condType, status, reason, message string) error {
	patch := fmt.Sprintf(`{"status":{"conditions":[{"type":"%s", "status":"%s", "reason":"%s", "message":"%s"}]}}`,
		condType, status, reason, message)
	return k.patchStatusStrategic(kind, name, patch)
}

func (k *Kubectl) patchStatusStrategic(kind, name, patch string) error {
	cmd, cancel := k.cmd("patch", kind, name, "--subresource=status", "--type", "strategic", "-p", patch)
	defer cancel()
	_, err := cmd.Output()
	return err
}

func (k *Kubectl) SetCordonAnnotation(nodeName string) ([]byte, error) {
	cmd, cancel := k.cmd("annotate", "node", nodeName, fmt.Sprintf("%s=%s", CordonAnnotationKey, CordonAnnotationValue))
	defer cancel()
	return cmd.Output()
}

func (k *Kubectl) RemoveCordonAnnotation(nodeName string) ([]byte, error) {
	cmd, cancel := k.cmd("annotate", "node", nodeName, fmt.Sprintf("%s-", CordonAnnotationKey))
	defer cancel()
	return cmd.Output()
}

func (k *Kubectl) GetCondition(nodeName, reason string) (*Condition, error) {
	out, err := k.getCondition(nodeName, reason)
	if err != nil {
		return nil, err
	}

	return ConditionFromJSON(out)
}

func (k *Kubectl) getCondition(nodeName, reason string) ([]byte, error) {
	jsonPath := fmt.Sprintf(`jsonpath={.status.conditions[?(@.reason=="%s")]}`, reason)
	cmd, cancel := k.cmd("get", "node", nodeName, "-o", jsonPath)
	defer cancel()
	return cmd.Output()
}

func (k *Kubectl) cmd(args ...string) (*exec.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	kArgs := append([]string{}, "--kubeconfig", KubeConfigPath)
	kArgs = append(kArgs, args...)
	return exec.CommandContext(ctx, k.kubectlPath, kArgs...), cancel
}
