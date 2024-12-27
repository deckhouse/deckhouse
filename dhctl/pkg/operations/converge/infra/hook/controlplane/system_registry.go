// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
	"github.com/hashicorp/go-multierror"
	labels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

const (
	registryDataDeviceMountLockAnnotation      = "embedded-registry.deckhouse.io/data-device-mount-lock"
	registryDataDeviceUnmountAllowedAnnotation = "embedded-registry.deckhouse.io/data-device-umount-allowed"
	registryDataDeviceUnmountDoneAnnotation    = "embedded-registry.deckhouse.io/data-device-umount-done"

	registryDataDeviceLabel      = "node.deckhouse.io/registry-data-device-ready"
	registryDataDeviceLabelValue = "true"

	registryPodsNamespace = "d8-system"
	registryModuleName    = "system-registry"

	registryTerraformEnableFlagVar = "systemRegistryEnable"

	NGCUmountTaskName    = "umount-registry-data-device-sh"
	NGCUmountTaskContent = `
NODE_NAME="%s"
UNMOUNT_ALLOWED_ANNOTATION="%s"
UNMOUNT_DONE_ANNOTATION="%s"

function check_annotation(){
    local annotation="$1"
    local node="$D8_NODE_HOSTNAME"
    local node_annotations=$(bb-kubectl --request-timeout=60s --kubeconfig=/etc/kubernetes/kubelet.conf get node $node -o json | jq '.metadata.annotations')

    if echo "$node_annotations" | jq 'has("'$annotation'")' | grep -q 'true'; then
        return 0
    fi
    return 1
}

function create_annotation(){
    local annotation="$1=\"\""
    local node="$D8_NODE_HOSTNAME"
    bb-kubectl --request-timeout=60s --kubeconfig=/etc/kubernetes/kubelet.conf annotate node $node --overwrite $annotation
}

function find_path_by_data_device_mountpoint() {
  local data_device_mountpoint="$1"
  lsblk -o path,type,mountpoint,fstype --tree --json | jq -r "
    [
      .blockdevices[] 
      | select(.mountpoint == \"$data_device_mountpoint\")  # Match the specific device mountpoint
      | .path
    ] | first"
}

function is_data_device_mounted() {
  local data_device_mountpoint="$1"
  local data_device
  data_device=$(find_path_by_data_device_mountpoint "$data_device_mountpoint")
  if [ "$data_device" != "null" ] && [ -n "$data_device" ]; then
    return 0
  else
    return 1
  fi
}

function teardown_registry_data_device() {
    local mount_point="/mnt/system-registry-data"
    local fstab_file="/etc/fstab"
    local link_target="/opt/deckhouse/system-registry"
    local label="registry-data"

    # Umount data device
    if is_data_device_mounted "$mount_point"; then
        umount $mount_point
    fi
    
    # Remove the entry from /etc/fstab
    if grep -q "$label" "$fstab_file"; then
        sed -i "/^LABEL=${label}.*/d" "$fstab_file"
    fi

    # Remove the mount point if it exists
    if [[ -e "$mount_point" ]]; then
        rm -rf "$mount_point"
    fi
  
    # Remove the symbolic link if it exists
    if [[ -L "$link_target" ]]; then
        rm -f "$link_target"
    fi
}

if [[ "$D8_NODE_HOSTNAME" != "$NODE_NAME" ]]; then
    exit 0
fi

if check_annotation "$UNMOUNT_ALLOWED_ANNOTATION"; then
    teardown_registry_data_device
    create_annotation "$UNMOUNT_DONE_ANNOTATION"
fi
`
)

var (
	NGCGVK = schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1alpha1",
		Kind:    "NodeGroupConfiguration",
	}
	NGCGVR = schema.GroupVersionResource{
		Group:    NGCGVK.Group,
		Version:  NGCGVK.Version,
		Resource: "nodegroupconfigurations",
	}
)

func isRegistryMustBeEnabled(terraformVars []byte) (bool, error) {
	var objmap map[string]*json.RawMessage
	if err := json.Unmarshal(terraformVars, &objmap); err != nil {
		return false, nil
	}

	value, found := objmap[registryTerraformEnableFlagVar]
	if !found {
		return false, nil
	}

	var boolValue bool
	if err := json.Unmarshal(*value, &boolValue); err != nil {
		return false, err
	}
	return boolValue, nil
}

func waitForRegistryPodsDeletion(kubeClient *client.KubernetesClient, nodeName string) error {
	return retry.NewLoop(
		fmt.Sprintf("Checking for registry pods on node '%s'", nodeName),
		45, 10*time.Second,
	).Run(func() error {
		var result *multierror.Error

		if err := checkPodsExistence(
			kubeClient, nodeName, registryPodsNamespace, "registry static",
			map[string]string{
				"component": "system-registry",
				"tier":      "control-plane",
			},
		); err != nil {
			result = multierror.Append(result, err)
		}

		if err := checkPodsExistence(
			kubeClient, nodeName, registryPodsNamespace, "registry static pod manager",
			map[string]string{
				"app": "system-registry-staticpod-manager",
			},
		); err != nil {
			result = multierror.Append(result, err)
		}

		if result.ErrorOrNil() != nil {
			result = multierror.Append(
				result,
				fmt.Errorf(
					"pods of module '%s' have been detected. Before disconnecting the disks, you need to disable module '%s'",
					registryModuleName,
					registryModuleName,
				),
			)
		}

		return result.ErrorOrNil()
	})
}

func tryLockRegistryDataDeviceMount(kubeClient *client.KubernetesClient, nodeName string) error {
	return retry.NewLoop(
		fmt.Sprintf("Attempting to lock mount actions for registry data device on node '%s'", nodeName),
		45, 10*time.Second,
	).Run(func() error {
		return setAnnotations(kubeClient, nodeName, map[string]string{registryDataDeviceMountLockAnnotation: ""})
	})
}

func tryUnlockRegistryDataDeviceMount(kubeClient *client.KubernetesClient, nodeName string) error {
	return retry.NewLoop(
		fmt.Sprintf("Attempting to unlock registry data device on node '%s'", nodeName),
		45, 10*time.Second,
	).Run(func() error {
		return unsetAnnotations(kubeClient, nodeName, []string{registryDataDeviceMountLockAnnotation})
	})
}

func setRegistryDataDeviceUnmountAnnotations(kubeClient *client.KubernetesClient, nodeName string) error {
	return retry.NewLoop(
		fmt.Sprintf("Attempting to set umount annotations on node '%s'", nodeName),
		45, 10*time.Second,
	).Run(func() error {
		return setAnnotations(kubeClient, nodeName, map[string]string{registryDataDeviceUnmountAllowedAnnotation: ""})
	})
}

func unsetRegistryDataDeviceUnmountAnnotations(kubeClient *client.KubernetesClient, nodeName string) error {
	return retry.NewLoop(
		fmt.Sprintf("Attempting to unset umount annotations on node '%s'", nodeName),
		45, 10*time.Second,
	).Run(func() error {
		return unsetAnnotations(
			kubeClient,
			nodeName,
			[]string{
				registryDataDeviceUnmountAllowedAnnotation,
				registryDataDeviceUnmountDoneAnnotation,
			},
		)
	})
}

func unsetRegistryDataDeviceNodeLabel(kubeClient *client.KubernetesClient, nodeName string) error {
	return retry.NewLoop(
		fmt.Sprintf("Attempting to unset DataDeviceNodeLabel '%s'", nodeName),
		45, 10*time.Second,
	).Run(func() error {
		return unsetLabels(kubeClient, nodeName, []string{registryDataDeviceLabel})
	})
}

func isExistRegistryDataDeviceUnmountDoneAnnotation(kubeClient *client.KubernetesClient, nodeName string) (bool, error) {
	isExist := false
	err := retry.NewLoop(
		fmt.Sprintf("Attempting to check RegistryDataDeviceUnmountDoneAnnotation '%s'", nodeName),
		45, 10*time.Second,
	).Run(func() error {
		var err error
		isExist, err = isAnnotationExist(kubeClient, nodeName, registryDataDeviceUnmountDoneAnnotation)
		if !isExist {
			return fmt.Errorf("Failed to wait...")
		}
		return err
	})
	return isExist, err
}

func isExistRegistryDataDeviceNodeLabel(kubeClient *client.KubernetesClient, nodeName string) (bool, error) {
	isExist := false
	err := retry.NewLoop(
		fmt.Sprintf("Attempting to check RegistryDataDeviceNodeLabel '%s'", nodeName),
		45, 10*time.Second,
	).Run(func() error {
		var err error
		isExist, err = isLabelExist(kubeClient, nodeName, registryDataDeviceLabel, registryDataDeviceLabelValue)
		if !isExist {
			return fmt.Errorf("Failed to wait...")
		}
		return err
	})
	return isExist, err
}

func checkPodsExistence(kubeClient *client.KubernetesClient, nodeName, podNamespace, podName string, podLabels map[string]string) error {
	staticPods, err := kubeClient.CoreV1().Pods(podNamespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labels.Set(podLabels).String(),
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		if errors.IsNotFound(err) {
			log.InfoF("No '%s' pod found on node '%s'. Skipping.", podName, nodeName)
			return nil
		}
		return fmt.Errorf("failed to list '%s' pod on node '%s': %v", podName, nodeName, err)
	}
	if len(staticPods.Items) > 0 {
		var podNames []string
		for _, pod := range staticPods.Items {
			podNames = append(podNames, pod.Name)
		}
		return fmt.Errorf(
			"found '%s' pod '%v' on node '%s'",
			podName,
			podNames,
			nodeName,
		)
	}
	return nil
}

func isAnnotationExist(kubeClient *client.KubernetesClient, nodeName, annotation string) (bool, error) {
	nodeObj, err := kubeClient.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get the node %s: %w", nodeName, err)
	}

	nodeAnnotations := nodeObj.GetAnnotations()
	_, ok := nodeAnnotations[annotation]
	return ok, nil
}

func setAnnotations(kubeClient *client.KubernetesClient, nodeName string, annotations map[string]string) error {
	nodeObj, err := kubeClient.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get the node %s: %w", nodeName, err)
	}

	nodeAnnotations := nodeObj.GetAnnotations()

	patchData := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{},
		},
	}

	for annotation, expectedValue := range annotations {
		// Check if the annotation exists
		if currentValue, ok := nodeAnnotations[annotation]; ok && expectedValue == currentValue {
			continue
		}
		patchData.ObjectMeta.Annotations[annotation] = expectedValue
	}

	if len(patchData.ObjectMeta.Annotations) == 0 {
		return nil
	}

	patch, err := json.Marshal(patchData)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	_, err = kubeClient.CoreV1().Nodes().Patch(
		context.Background(),
		nodeName,
		types.MergePatchType,
		patch,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to set annotations for the node: %w", err)
	}
	return nil
}

func unsetAnnotations(kubeClient *client.KubernetesClient, nodeName string, annotations []string) error {
	nodeObj, err := kubeClient.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get the node %s: %w", nodeName, err)
	}

	nodeAnnotations := nodeObj.GetAnnotations()

	patchOperations := make([]map[string]interface{}, 0, len(annotations))

	for _, annotation := range annotations {
		// Check if the annotation exists
		if _, ok := nodeAnnotations[annotation]; !ok {
			continue
		}

		patchOperations = append(patchOperations, map[string]interface{}{
			"op": "remove",
			// JSON patch requires slashes to be escaped with ~1
			"path": fmt.Sprintf("/metadata/annotations/%s", strings.ReplaceAll(annotation, "/", "~1")),
		})
	}

	if len(patchOperations) == 0 {
		return nil
	}

	patch, err := json.Marshal(patchOperations)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	_, err = kubeClient.CoreV1().Nodes().Patch(
		context.Background(),
		nodeName,
		types.JSONPatchType,
		patch,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to unset annotations for the node: %w", err)
	}
	return nil
}

func isLabelExist(kubeClient *client.KubernetesClient, nodeName, label, labelValue string) (bool, error) {
	nodeObj, err := kubeClient.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get the node %s: %w", nodeName, err)
	}

	nodeLabels := nodeObj.GetLabels()
	value, ok := nodeLabels[label]
	return ok && value == labelValue, nil
}

func unsetLabels(kubeClient *client.KubernetesClient, nodeName string, labels []string) error {
	nodeObj, err := kubeClient.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get the node %s: %w", nodeName, err)
	}

	nodeLabels := nodeObj.GetLabels()

	patchOperations := make([]map[string]interface{}, 0, len(labels))

	for _, label := range labels {
		// Check if the label exists
		if _, ok := nodeLabels[label]; !ok {
			continue
		}

		patchOperations = append(patchOperations, map[string]interface{}{
			"op": "remove",
			// JSON patch requires slashes to be escaped with ~1
			"path": fmt.Sprintf("/metadata/labels/%s", strings.ReplaceAll(label, "/", "~1")),
		})
	}

	if len(patchOperations) == 0 {
		return nil
	}

	patch, err := json.Marshal(patchOperations)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	_, err = kubeClient.CoreV1().Nodes().Patch(
		context.Background(),
		nodeName,
		types.JSONPatchType,
		patch,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to unset labels for the node: %w", err)
	}
	time.Sleep(5 * time.Second)
	return nil
}

func createOrUpdateNGCUmountTask(kubeClient *client.KubernetesClient, nodeName string) error {
	return createOrUpdateNGCUmountTaskWithContent(
		kubeClient,
		createNGCUmountTaskContent(
			nodeName,
			registryDataDeviceUnmountAllowedAnnotation,
			registryDataDeviceUnmountDoneAnnotation,
		),
	)
}

func createNGCUmountTaskContent(nodeName, unmountAllowedAnnotation, unmountDoneAnnotation string) string {
	return fmt.Sprintf(NGCUmountTaskContent, nodeName, unmountAllowedAnnotation, unmountDoneAnnotation)
}

func createOrUpdateNGCUmountTaskWithContent(kubeClient *client.KubernetesClient, content string) error {
	const retryAttempts = 45
	const retryInterval = 10 * time.Second

	// Create a new NodeGroupConfiguration object
	newObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": NGCGVK.GroupVersion().String(),
			"kind":       NGCGVK.Kind,
			"metadata": map[string]interface{}{
				"name": NGCUmountTaskName,
			},
			"spec": map[string]interface{}{
				"weight":     0,
				"nodeGroups": []string{"master"},
				"bundles":    []string{"*"},
				"content":    content,
			},
		},
	}

	return retry.NewLoop(
		fmt.Sprintf(`Attempting to create/update NodeGroupConfiguration "%s"`, NGCUmountTaskName),
		retryAttempts, retryInterval,
	).Run(func() error {
		currentObj, err := kubeClient.Dynamic().Resource(NGCGVR).
			Get(context.TODO(), NGCUmountTaskName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				_, createErr := kubeClient.Dynamic().Resource(NGCGVR).
					Create(context.TODO(), newObj, metav1.CreateOptions{})
				return createErr
			}
			return err
		}

		newObj.SetResourceVersion(currentObj.GetResourceVersion())
		_, updateErr := kubeClient.Dynamic().Resource(NGCGVR).
			Update(context.TODO(), newObj, metav1.UpdateOptions{})
		return updateErr
	})
}

func removeNGCUmountTask(kubeClient *client.KubernetesClient) error {
	const retryAttempts = 45
	const retryInterval = 10 * time.Second

	return retry.NewLoop(
		fmt.Sprintf(`Attempting to delete NodeGroupConfiguration "%s"`, NGCUmountTaskName),
		retryAttempts, retryInterval,
	).Run(func() error {
		err := kubeClient.Dynamic().Resource(NGCGVR).
			Delete(context.TODO(), NGCUmountTaskName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		return nil
	})
}
