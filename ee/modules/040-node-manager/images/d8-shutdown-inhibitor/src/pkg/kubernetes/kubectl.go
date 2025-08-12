/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package kubernetes

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
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
	cmd := k.cmd("cordon", nodeName)
	return cmd.CombinedOutput()
}

func (k *Kubectl) Uncordon(nodeName string) ([]byte, error) {
	cmd := k.cmd("uncordon", nodeName)
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
	cmd := k.cmd("get", "node", nodeName, "-o", jsonPath)
	return cmd.Output()
}

func (k *Kubectl) listPods(nodeName string) ([]byte, error) {
	nodeNameFieldSelector := fmt.Sprintf("spec.nodeName=%s", nodeName)
	cmd := k.cmd("get", "po", "-A", "-o", "json", "--field-selector", nodeNameFieldSelector)
	return cmd.Output()
}

func (k *Kubectl) PatchCondition(kind, name, condType, status, reason, message string) error {
	patch := fmt.Sprintf(`{"status":{"conditions":[{"type":"%s", "status":"%s", "reason":"%s", "message":"%s"}]}}`,
		condType, status, reason, message)
	return k.patchStatusStrategic(kind, name, patch)
}

func (k *Kubectl) patchStatusStrategic(kind, name, patch string) error {
	cmd := k.cmd("patch", kind, name, "--subresource=status", "--type", "strategic", "-p", patch)
	_, err := cmd.Output()
	return err
}

func (k *Kubectl) SetCordonAnnotation(nodeName string) ([]byte, error) {
	cmd := k.cmd("annotate", "node", nodeName, fmt.Sprintf("%s=%s", CordonAnnotationKey, CordonAnnotationValue))
	return cmd.Output()
}

func (k *Kubectl) RemoveCordonAnnotation(nodeName string) ([]byte, error) {
	cmd := k.cmd("annotate", "node", nodeName, fmt.Sprintf("%s-", CordonAnnotationKey))
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
	cmd := k.cmd("get", "node", nodeName, "-o", jsonPath)
	return cmd.Output()
}

func (k *Kubectl) cmd(args ...string) *exec.Cmd {
	kArgs := append([]string{}, "--kubeconfig", KubeConfigPath)
	kArgs = append(kArgs, args...)
	return exec.Command(k.kubectlPath, kArgs...)
}
