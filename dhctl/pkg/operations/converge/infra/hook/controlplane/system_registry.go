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

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
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

	NgcUmountTaskName = "umount-registry-data-device"

	RegistryDataDeviceEnableTerraformVar = config.RegistryDataDeviceEnableTerraformVar
)

var (
	NgcGVK = schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1alpha1",
		Kind:    "NodeGroupConfiguration",
	}
	NgcGVR = schema.GroupVersionResource{
		Group:    NgcGVK.Group,
		Version:  NgcGVK.Version,
		Resource: "nodegroupconfigurations",
	}
)

func isRegistryMustBeEnabled(terraformVars []byte) (bool, error) {
	var objmap map[string]*json.RawMessage
	if err := json.Unmarshal(terraformVars, &objmap); err != nil {
		return false, nil
	}

	value, found := objmap[RegistryDataDeviceEnableTerraformVar]
	if !found {
		return false, nil
	}

	var boolValue bool
	if err := json.Unmarshal(*value, &boolValue); err != nil {
		return false, err
	}
	return boolValue, nil
}

func waitForNoRegistryPodsOnNode(ctx context.Context, kubeClient *client.KubernetesClient, nodeName string) error {
	loopName := fmt.Sprintf("Check registry pods on node '%s'", nodeName)
	const loopRetryAttempts = 45
	const loopRetryInterval = 10 * time.Second

	registryPods := []struct {
		description string
		labels      map[string]string
	}{
		{
			description: "registry static",
			labels: map[string]string{
				"component": "system-registry",
				"tier":      "control-plane",
			},
		},
		{
			description: "registry static pod manager",
			labels: map[string]string{
				"app": "system-registry-staticpod-manager",
			},
		},
	}

	return retry.NewLoop(loopName, loopRetryAttempts, loopRetryInterval).
		WithContext(ctx).
		Run(func() error {
			var result *multierror.Error

			for _, pod := range registryPods {
				if err := checkPodsExistence(
					ctx,
					kubeClient,
					nodeName,
					registryPodsNamespace,
					pod.description,
					pod.labels,
				); err != nil {
					result = multierror.Append(result, err)
				}
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

func createNgcForUmountingRegistryDataDevice(ctx context.Context, kubeClient *client.KubernetesClient, nodeName string) error {
	loopName := fmt.Sprintf("Create NodeGroupConfiguration '%s'", NgcUmountTaskName)
	const loopRetryAttempts = 45
	const loopRetryInterval = 10 * time.Second

	renderedTemplate, err := template.RenderUmountRegistryDataDeviceStep(
		nodeName,
		registryDataDeviceUnmountAllowedAnnotation,
		registryDataDeviceUnmountDoneAnnotation,
	)
	if err != nil {
		return err
	}

	// Create a new NodeGroupConfiguration object
	newObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": NgcGVK.GroupVersion().String(),
			"kind":       NgcGVK.Kind,
			"metadata": map[string]interface{}{
				"name": NgcUmountTaskName,
			},
			"spec": map[string]interface{}{
				"weight":     0,
				"nodeGroups": []string{"master"},
				"bundles":    []string{"*"},
				"content":    renderedTemplate.Content.String(),
			},
		},
	}

	return retry.NewLoop(loopName, loopRetryAttempts, loopRetryInterval).
		WithContext(ctx).
		Run(func() error {
			// Prepare annotation
			err := manageNodeAnnotations(
				ctx,
				kubeClient,
				nodeName,
				// Create annotation
				map[string]string{registryDataDeviceUnmountAllowedAnnotation: ""},
				// Delete annotation
				[]string{registryDataDeviceUnmountDoneAnnotation},
			)
			if err != nil {
				return err
			}

			// Get current NGC
			currentObj, err := kubeClient.
				Dynamic().
				Resource(NgcGVR).
				Get(ctx, NgcUmountTaskName, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					// Create if not exist
					_, createErr := kubeClient.
						Dynamic().
						Resource(NgcGVR).
						Create(ctx, newObj, metav1.CreateOptions{})
					return createErr
				}
				return err
			}

			// Update if NGC exist
			newObj.SetResourceVersion(currentObj.GetResourceVersion())
			_, updateErr := kubeClient.Dynamic().Resource(NgcGVR).
				Update(ctx, newObj, metav1.UpdateOptions{})
			return updateErr
		})
}

func deleteNgcForUmountingRegistryDataDevice(ctx context.Context, kubeClient *client.KubernetesClient, nodeName string) error {
	loopName := fmt.Sprintf("Delete NodeGroupConfiguration '%s'", NgcUmountTaskName)
	const loopRetryAttempts = 45
	const loopRetryInterval = 10 * time.Second

	return retry.NewLoop(loopName, loopRetryAttempts, loopRetryInterval).
		WithContext(ctx).
		Run(func() error {
			// Prepare annotation
			manageNodeAnnotations(
				ctx,
				kubeClient,
				nodeName,
				// Create annotation
				map[string]string{},
				// Delete annotation
				[]string{
					registryDataDeviceUnmountAllowedAnnotation,
					registryDataDeviceUnmountDoneAnnotation,
				},
			)

			// Delete NGC if not exist
			err := kubeClient.
				Dynamic().
				Resource(NgcGVR).
				Delete(ctx, NgcUmountTaskName, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			return nil
		})
}

func waitDoneNgcForUmountingRegistryDataDevice(ctx context.Context, kubeClient *client.KubernetesClient, nodeName string) error {
	loopName := fmt.Sprintf("Wait for '%s' to unmount registry data device", nodeName)
	const loopRetryAttempts = 100
	const loopRetryInterval = 10 * time.Second

	var isExist bool
	err := retry.NewLoop(loopName, loopRetryAttempts, loopRetryInterval).
		WithContext(ctx).
		Run(func() error {
			var err error
			isExist, err = isAnnotationExist(ctx, kubeClient, nodeName, registryDataDeviceUnmountDoneAnnotation)
			if err != nil {
				return fmt.Errorf("failed to check unmount done annotation for node '%s': %v", nodeName, err)
			}
			if !isExist {
				return fmt.Errorf("waiting for the registry data device to be unmounted on node '%s'", nodeName)
			}
			return nil
		})
	return err
}

func isRegistryDataDeviceExistOnNode(ctx context.Context, kubeClient *client.KubernetesClient, nodeName string) (bool, error) {
	loopName := fmt.Sprintf("Check if registry data device exists on node '%s'", nodeName)
	const loopRetryAttempts = 45
	const loopRetryInterval = 10 * time.Second

	var isExist bool
	err := retry.NewLoop(loopName, loopRetryAttempts, loopRetryInterval).
		WithContext(ctx).
		Run(func() error {
			var checkErr error
			isExist, checkErr = isLabelExist(ctx, kubeClient, nodeName, registryDataDeviceLabel, registryDataDeviceLabelValue)
			return checkErr
		})

	return isExist, err
}

func lockRegistryDataDeviceMount(ctx context.Context, kubeClient *client.KubernetesClient, nodeName string) error {
	loopName := fmt.Sprintf("Lock mount actions for registry data device on node '%s'", nodeName)
	const loopRetryAttempts = 45
	const loopRetryInterval = 10 * time.Second

	return retry.NewLoop(loopName, loopRetryAttempts, loopRetryInterval).
		WithContext(ctx).
		Run(func() error {
			return manageNodeAnnotations(ctx, kubeClient, nodeName, map[string]string{registryDataDeviceMountLockAnnotation: ""}, []string{})
		})
}

func unlockRegistryDataDeviceMount(ctx context.Context, kubeClient *client.KubernetesClient, nodeName string) error {
	loopName := fmt.Sprintf("Ulock mount actions for registry data device on node '%s'", nodeName)
	const loopRetryAttempts = 45
	const loopRetryInterval = 10 * time.Second

	return retry.NewLoop(loopName, loopRetryAttempts, loopRetryInterval).
		WithContext(ctx).
		Run(func() error {
			return manageNodeAnnotations(ctx, kubeClient, nodeName, map[string]string{}, []string{registryDataDeviceMountLockAnnotation})
		})
}

func unsetRegistryDataDeviceNodeLabel(ctx context.Context, kubeClient *client.KubernetesClient, nodeName string) error {
	loopName := fmt.Sprintf("Remove registry data device labels from node '%s'", nodeName)
	const loopRetryAttempts = 45
	const loopRetryInterval = 10 * time.Second

	return retry.NewLoop(loopName, loopRetryAttempts, loopRetryInterval).
		WithContext(ctx).
		Run(func() error {
			return manageNodeLabels(ctx, kubeClient, nodeName, map[string]string{}, []string{registryDataDeviceLabel})
		})
}

func checkPodsExistence(
	ctx context.Context,
	kubeClient *client.KubernetesClient,
	nodeName,
	podNamespace,
	podDescription string,
	podLabels map[string]string,
) error {
	staticPods, err := kubeClient.CoreV1().Pods(podNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(podLabels).String(),
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})

	// Handle error if listing pods fails
	if err != nil {
		if errors.IsNotFound(err) {
			log.InfoF("No '%s' pod found on node '%s'. Skipping.", podDescription, nodeName)
			return nil // No pods found, but it's not an error
		}
		return fmt.Errorf("failed to list '%s' pod on node '%s': %v", podDescription, nodeName, err)
	}

	// If pods are found, log them as a problem
	if len(staticPods.Items) > 0 {
		var podNames []string
		for _, pod := range staticPods.Items {
			podNames = append(podNames, pod.Name)
		}
		return fmt.Errorf(
			"found '%s' pod(s) %v on node '%s'",
			podDescription,
			podNames,
			nodeName,
		)
	}

	// No matching pods found, so nothing to report
	return nil
}

func isAnnotationExist(ctx context.Context, kubeClient *client.KubernetesClient, nodeName, annotation string) (bool, error) {
	nodeObj, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get the node %s: %w", nodeName, err)
	}

	nodeAnnotations := nodeObj.GetAnnotations()
	_, ok := nodeAnnotations[annotation]
	return ok, nil
}

func isLabelExist(ctx context.Context, kubeClient *client.KubernetesClient, nodeName, label, labelValue string) (bool, error) {
	nodeObj, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get the node %s: %w", nodeName, err)
	}

	nodeLabels := nodeObj.GetLabels()
	value, ok := nodeLabels[label]
	return ok && value == labelValue, nil
}

func manageNodeAnnotations(ctx context.Context, kubeClient *client.KubernetesClient, nodeName string, annotationsToSet map[string]string, annotationsToUnset []string) error {
	// Check for conflicts between annotationsToSet and annotationsToUnset
	unsetMap := make(map[string]struct{}, len(annotationsToUnset))
	for _, annotation := range annotationsToUnset {
		unsetMap[annotation] = struct{}{}
	}

	for annotation := range annotationsToSet {
		if _, exists := unsetMap[annotation]; exists {
			return fmt.Errorf("conflict: annotation %q exists in both set and unset lists", annotation)
		}
	}

	nodeObj, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get the node %q: %w", nodeName, err)
	}

	nodeAnnotations := nodeObj.GetAnnotations()

	var patchOperations []map[string]interface{}

	// Add or update annotations
	for annotation, expectedValue := range annotationsToSet {
		if currentValue, exists := nodeAnnotations[annotation]; exists && currentValue == expectedValue {
			continue
		}
		patchOperations = append(patchOperations, map[string]interface{}{
			"op":    "add",
			"path":  fmt.Sprintf("/metadata/annotations/%s", strings.ReplaceAll(annotation, "/", "~1")),
			"value": expectedValue,
		})
	}

	// Remove annotations
	for _, annotation := range annotationsToUnset {
		if _, exists := nodeAnnotations[annotation]; !exists {
			continue
		}
		patchOperations = append(patchOperations, map[string]interface{}{
			"op":   "remove",
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
		ctx,
		nodeName,
		types.JSONPatchType,
		patch,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to apply annotations patch to the node %q: %w", nodeName, err)
	}

	return nil
}

func manageNodeLabels(ctx context.Context, kubeClient *client.KubernetesClient, nodeName string, labelsToSet map[string]string, labelsToUnset []string) error {
	// Check for conflicts between labelsToSet and labelsToUnset
	unsetMap := make(map[string]struct{}, len(labelsToUnset))
	for _, label := range labelsToUnset {
		unsetMap[label] = struct{}{}
	}

	for label := range labelsToSet {
		if _, exists := unsetMap[label]; exists {
			return fmt.Errorf("conflict: label %q exists in both set and unset lists", label)
		}
	}

	nodeObj, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get the node %q: %w", nodeName, err)
	}

	nodeLabels := nodeObj.GetLabels()

	var patchOperations []map[string]interface{}

	// Add or update labels
	for label, expectedValue := range labelsToSet {
		if currentValue, exists := nodeLabels[label]; exists && currentValue == expectedValue {
			continue
		}
		patchOperations = append(patchOperations, map[string]interface{}{
			"op":    "add",
			"path":  fmt.Sprintf("/metadata/labels/%s", strings.ReplaceAll(label, "/", "~1")),
			"value": expectedValue,
		})
	}

	// Remove labels
	for _, label := range labelsToUnset {
		if _, exists := nodeLabels[label]; !exists {
			continue
		}
		patchOperations = append(patchOperations, map[string]interface{}{
			"op":   "remove",
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
		ctx,
		nodeName,
		types.JSONPatchType,
		patch,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to apply labels patch to the node %q: %w", nodeName, err)
	}

	return nil
}
