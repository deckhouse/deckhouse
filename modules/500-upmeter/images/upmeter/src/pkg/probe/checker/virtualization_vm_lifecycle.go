/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package checker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

const (
	virtualizationProbeName = "virtualization"
	virtualizationVMName    = "probe-vm"
	virtualizationDiskName  = "probe-disk"
	virtualizationEvictName = "probe-vm-evict"

	virtualizationPhaseReady   = "Ready"
	virtualizationPhaseRunning = "Running"
	vmopPhaseCompleted         = "Completed"
	vmopPhaseFailed            = "Failed"
	vmopTypeEvict              = "Evict"

	virtualizationConditionAgentReady = "AgentReady"
	conditionStatusTrue               = "True"

	defaultVMClassAnnotation = "virtualmachineclass.virtualization.deckhouse.io/is-default-class"
)

var (
	virtualImageGVR = schema.GroupVersionResource{
		Group:    "virtualization.deckhouse.io",
		Version:  "v1alpha2",
		Resource: "virtualimages",
	}
	virtualDiskGVR = schema.GroupVersionResource{
		Group:    "virtualization.deckhouse.io",
		Version:  "v1alpha2",
		Resource: "virtualdisks",
	}
	virtualMachineGVR = schema.GroupVersionResource{
		Group:    "virtualization.deckhouse.io",
		Version:  "v1alpha2",
		Resource: "virtualmachines",
	}
	virtualMachineOperationGVR = schema.GroupVersionResource{
		Group:    "virtualization.deckhouse.io",
		Version:  "v1alpha2",
		Resource: "virtualmachineoperations",
	}
	virtualMachineClassGVR = schema.GroupVersionResource{
		Group:    "virtualization.deckhouse.io",
		Version:  "v1alpha3",
		Resource: "virtualmachineclasses",
	}
)

// VirtualMachineLifecycle is a checker constructor and configurator.
type VirtualMachineLifecycle struct {
	Access           kubernetes.Access
	PreflightChecker check.Checker
	Logger           *logrus.Entry

	AgentID          string
	Namespace        string
	VirtualImageName string
	VirtualImageURL  string

	RequestTimeout                     time.Duration
	WaitVirtualImageTimeout            time.Duration
	WaitVirtualDiskTimeout             time.Duration
	WaitVirtualMachineTimeout          time.Duration
	WaitVirtualMachineMigrationTimeout time.Duration
	WaitDeletionTimeout                time.Duration
	WaitNamespaceDeletedTimeout        time.Duration
	Timeout                            time.Duration
}

func (c VirtualMachineLifecycle) Checker() check.Checker {
	checker := &virtualMachineLifecycleChecker{
		access:           c.Access,
		preflightChecker: c.PreflightChecker,
		logger:           c.Logger,
		agentID:          fallbackString(c.AgentID, "unknown"),
		namespace:        c.Namespace,
		virtualImageName: c.VirtualImageName,
		virtualImageURL:  c.VirtualImageURL,

		requestTimeout:                     fallbackDuration(c.RequestTimeout, 5*time.Second),
		waitVirtualImageTimeout:            fallbackDuration(c.WaitVirtualImageTimeout, 15*time.Minute),
		waitVirtualDiskTimeout:             fallbackDuration(c.WaitVirtualDiskTimeout, 3*time.Minute),
		waitVirtualMachineTimeout:          fallbackDuration(c.WaitVirtualMachineTimeout, 5*time.Minute),
		waitVirtualMachineMigrationTimeout: fallbackDuration(c.WaitVirtualMachineMigrationTimeout, time.Minute),
		waitDeletionTimeout:                fallbackDuration(c.WaitDeletionTimeout, 2*time.Minute),
		waitNamespaceDeletedTimeout:        fallbackDuration(c.WaitNamespaceDeletedTimeout, 2*time.Minute),
	}

	return withTimeout(checker, fallbackDuration(c.Timeout, 25*time.Minute))
}

type virtualMachineLifecycleChecker struct {
	access           kubernetes.Access
	preflightChecker check.Checker
	logger           *logrus.Entry

	agentID          string
	namespace        string
	virtualImageName string
	virtualImageURL  string

	requestTimeout                     time.Duration
	waitVirtualImageTimeout            time.Duration
	waitVirtualDiskTimeout             time.Duration
	waitVirtualMachineTimeout          time.Duration
	waitVirtualMachineMigrationTimeout time.Duration
	waitDeletionTimeout                time.Duration
	waitNamespaceDeletedTimeout        time.Duration
}

func (c *virtualMachineLifecycleChecker) Check() check.Error {
	ctx := context.Background()

	if err := c.runCheckStep("preflight", c.preflight); err != nil {
		return err
	}

	var hasGarbage bool
	if err := c.runStep("checking garbage", func() error {
		var checkErr error
		hasGarbage, checkErr = c.hasGarbage(ctx)
		return checkErr
	}); err != nil {
		return check.ErrUnknown("checking garbage: %v", err)
	}
	if hasGarbage {
		if cleanupErr := c.runStep("cleaning garbage", func() error { return c.cleanup(ctx) }); cleanupErr != nil {
			return check.ErrUnknown("cleaning garbage: %v", cleanupErr)
		}
		return check.ErrUnknown("cleaned garbage")
	}

	result := c.doLifecycle(ctx)
	return wrapCleanupResult(result, c.cleanup(ctx))
}

func (c *virtualMachineLifecycleChecker) preflight() check.Error {
	if c.preflightChecker != nil {
		if err := c.preflightChecker.Check(); err != nil {
			return check.ErrUnknown("preflight: %v", err)
		}
	}
	return nil
}

func (c *virtualMachineLifecycleChecker) doLifecycle(ctx context.Context) check.Error {
	if err := c.runStep("creating namespace", func() error { return c.createNamespace(ctx) }); err != nil {
		return lifecycleStepError("creating namespace", err)
	}

	if err := c.runStep("ensuring VirtualImage", func() error {
		return c.ensureVirtualImageReady(ctx)
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail(
				"verification: VirtualImage %q is not Ready",
				c.virtualImageName,
			)
		}
		return lifecycleStepError("ensuring VirtualImage", err)
	}

	if err := c.runStep("creating VirtualDisk", func() error { return c.createVirtualDisk(ctx) }); err != nil {
		return lifecycleStepError("creating VirtualDisk", err)
	}

	if err := c.runStep("waiting for VirtualDisk", func() error { return c.waitVirtualDiskReady(ctx) }); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: VirtualDisk did not become Ready")
		}
		return lifecycleStepError("waiting for VirtualDisk", err)
	}

	if err := c.runStep("creating VirtualMachine", func() error { return c.createVirtualMachine(ctx) }); err != nil {
		return lifecycleStepError("creating VirtualMachine", err)
	}

	if err := c.runStep("waiting for VirtualMachine Running", func() error {
		return c.waitVirtualMachineRunning(ctx)
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: VirtualMachine did not reach Running phase")
		}
		return lifecycleStepError("waiting for VirtualMachine Running", err)
	}

	if err := c.runStep("waiting for VirtualMachine AgentReady", func() error {
		return c.waitVirtualMachineAgentReady(ctx)
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: VirtualMachine AgentReady condition did not become True")
		}
		return lifecycleStepError("waiting for VirtualMachine AgentReady", err)
	}

	if err := c.runStep("checking VirtualMachine migration", func() error {
		return c.verifyVirtualMachineMigration(ctx)
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: VirtualMachine migration did not complete")
		}
		return lifecycleStepError("checking VirtualMachine migration", err)
	}

	if err := c.runStep("deleting VirtualMachine", func() error {
		err := c.deleteVirtualMachine(ctx)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}); err != nil {
		return lifecycleStepError("deleting VirtualMachine", err)
	}

	if err := c.runStep("waiting for VirtualMachine deletion", func() error {
		return c.waitVirtualMachineAbsent(ctx)
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: VirtualMachine was not deleted")
		}
		return lifecycleStepError("waiting for VirtualMachine deletion", err)
	}

	if err := c.runStep("deleting VirtualDisk", func() error {
		err := c.deleteVirtualDisk(ctx)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}); err != nil {
		return lifecycleStepError("deleting VirtualDisk", err)
	}

	if err := c.runStep("waiting for VirtualDisk deletion", func() error {
		return c.waitVirtualDiskAbsent(ctx)
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: VirtualDisk was not deleted")
		}
		return lifecycleStepError("waiting for VirtualDisk deletion", err)
	}

	if err := c.runStep("deleting VirtualImage", func() error {
		err := c.deleteVirtualImage(ctx)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}); err != nil {
		return lifecycleStepError("deleting VirtualImage", err)
	}

	if err := c.runStep("waiting for VirtualImage deletion", func() error {
		return c.waitVirtualImageAbsent(ctx)
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: VirtualImage was not deleted")
		}
		return lifecycleStepError("waiting for VirtualImage deletion", err)
	}

	return nil
}

func (c *virtualMachineLifecycleChecker) runStep(step string, fn func() error) error {
	start := time.Now()
	err := fn()
	c.logStepDuration(step, time.Since(start))
	return err
}

func (c *virtualMachineLifecycleChecker) runCheckStep(step string, fn func() check.Error) check.Error {
	start := time.Now()
	err := fn()
	c.logStepDuration(step, time.Since(start))
	return err
}

func (c *virtualMachineLifecycleChecker) logStepDuration(step string, duration time.Duration) {
	if c.logger == nil {
		return
	}

	c.logger.WithFields(logrus.Fields{
		"step":     step,
		"duration": duration,
	}).Info("virtualization probe step completed")
}

func (c *virtualMachineLifecycleChecker) createNamespace(ctx context.Context) error {
	ns := &v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: c.namespace,
			Labels: map[string]string{
				"heritage":      "upmeter",
				agentLabelKey:   c.agentID,
				"upmeter-group": "extensions",
				"upmeter-probe": virtualizationProbeName,
			},
		},
	}
	_, err := c.access.Kubernetes().CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	return err
}

func (c *virtualMachineLifecycleChecker) deleteNamespace(ctx context.Context) error {
	return c.access.Kubernetes().CoreV1().Namespaces().Delete(ctx, c.namespace, metav1.DeleteOptions{})
}

func (c *virtualMachineLifecycleChecker) ensureVirtualImageReady(ctx context.Context) error {
	if err := c.createVirtualImageIfMissing(ctx); err != nil {
		return err
	}
	return c.waitVirtualImageReady(ctx)
}

func (c *virtualMachineLifecycleChecker) createVirtualImageIfMissing(ctx context.Context) error {
	_, err := c.access.Kubernetes().Dynamic().
		Resource(virtualImageGVR).
		Namespace(c.namespace).
		Get(ctx, c.virtualImageName, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	if c.virtualImageURL == "" {
		return fmt.Errorf("VirtualImage %q not found and virtualImageURL is not configured", c.virtualImageName)
	}

	manifest := virtualImageManifest(c.agentID, c.namespace, c.virtualImageName, c.virtualImageURL)
	obj, err := decodeManifestToUnstructured(manifest)
	if err != nil {
		return err
	}
	_, err = c.access.Kubernetes().Dynamic().
		Resource(virtualImageGVR).
		Namespace(c.namespace).
		Create(ctx, obj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (c *virtualMachineLifecycleChecker) waitVirtualImageReady(ctx context.Context) error {
	return waitForCondition(
		c.waitVirtualImageTimeout,
		pollingInterval(c.waitVirtualImageTimeout),
		func() (bool, error) {
			obj, err := c.access.Kubernetes().Dynamic().
				Resource(virtualImageGVR).
				Namespace(c.namespace).
				Get(ctx, c.virtualImageName, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return false, fmt.Errorf("VirtualImage %q not found", c.virtualImageName)
			}
			if err != nil {
				return false, err
			}
			return unstructuredNestedString(obj.Object, "status", "phase") == virtualizationPhaseReady, nil
		},
	)
}

func (c *virtualMachineLifecycleChecker) createVirtualDisk(ctx context.Context) error {
	manifest := virtualDiskManifest(c.agentID, c.namespace, virtualizationDiskName, c.virtualImageName)
	obj, err := decodeManifestToUnstructured(manifest)
	if err != nil {
		return err
	}
	_, err = c.access.Kubernetes().Dynamic().
		Resource(virtualDiskGVR).
		Namespace(c.namespace).
		Create(ctx, obj, metav1.CreateOptions{})
	return err
}

func (c *virtualMachineLifecycleChecker) deleteVirtualDisk(ctx context.Context) error {
	return c.access.Kubernetes().Dynamic().
		Resource(virtualDiskGVR).
		Namespace(c.namespace).
		Delete(ctx, virtualizationDiskName, metav1.DeleteOptions{})
}

func (c *virtualMachineLifecycleChecker) waitVirtualDiskReady(ctx context.Context) error {
	return c.waitResourcePhase(
		ctx,
		virtualDiskGVR,
		virtualizationDiskName,
		virtualizationPhaseReady,
		c.waitVirtualDiskTimeout,
	)
}

func (c *virtualMachineLifecycleChecker) waitVirtualDiskAbsent(ctx context.Context) error {
	return c.waitResourceAbsent(ctx, virtualDiskGVR, virtualizationDiskName, c.waitDeletionTimeout)
}

func (c *virtualMachineLifecycleChecker) deleteVirtualImage(ctx context.Context) error {
	return c.access.Kubernetes().Dynamic().
		Resource(virtualImageGVR).
		Namespace(c.namespace).
		Delete(ctx, c.virtualImageName, metav1.DeleteOptions{})
}

func (c *virtualMachineLifecycleChecker) waitVirtualImageAbsent(ctx context.Context) error {
	return c.waitResourceAbsent(ctx, virtualImageGVR, c.virtualImageName, c.waitDeletionTimeout)
}

func (c *virtualMachineLifecycleChecker) createVirtualMachine(ctx context.Context) error {
	manifest := virtualMachineManifest(
		c.agentID,
		c.namespace,
		virtualizationVMName,
		virtualizationDiskName,
	)
	obj, err := decodeManifestToUnstructured(manifest)
	if err != nil {
		return err
	}
	_, err = c.access.Kubernetes().Dynamic().
		Resource(virtualMachineGVR).
		Namespace(c.namespace).
		Create(ctx, obj, metav1.CreateOptions{})
	return err
}

func (c *virtualMachineLifecycleChecker) createVirtualMachineEvictOperation(ctx context.Context) error {
	manifest := virtualMachineOperationManifest(c.agentID, c.namespace, virtualizationEvictName, virtualizationVMName)
	obj, err := decodeManifestToUnstructured(manifest)
	if err != nil {
		return err
	}
	_, err = c.access.Kubernetes().Dynamic().
		Resource(virtualMachineOperationGVR).
		Namespace(c.namespace).
		Create(ctx, obj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (c *virtualMachineLifecycleChecker) deleteVirtualMachine(ctx context.Context) error {
	return c.access.Kubernetes().Dynamic().
		Resource(virtualMachineGVR).
		Namespace(c.namespace).
		Delete(ctx, virtualizationVMName, metav1.DeleteOptions{})
}

func (c *virtualMachineLifecycleChecker) waitVirtualMachineRunning(ctx context.Context) error {
	return c.waitResourcePhase(
		ctx,
		virtualMachineGVR,
		virtualizationVMName,
		virtualizationPhaseRunning,
		c.waitVirtualMachineTimeout,
	)
}

func (c *virtualMachineLifecycleChecker) waitVirtualMachineAgentReady(ctx context.Context) error {
	return c.waitResourceCondition(
		ctx,
		virtualMachineGVR,
		virtualizationVMName,
		virtualizationConditionAgentReady,
		conditionStatusTrue,
		c.waitVirtualMachineTimeout,
	)
}

func (c *virtualMachineLifecycleChecker) waitVirtualMachineMigrationCompleted(ctx context.Context, initialNode string) error {
	return waitForCondition(
		c.waitVirtualMachineMigrationTimeout,
		pollingInterval(c.waitVirtualMachineMigrationTimeout),
		func() (bool, error) {
			vmop, err := c.access.Kubernetes().Dynamic().
				Resource(virtualMachineOperationGVR).
				Namespace(c.namespace).
				Get(ctx, virtualizationEvictName, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			if err != nil {
				return false, err
			}

			phase := unstructuredNestedString(vmop.Object, "status", "phase")
			if phase == vmopPhaseFailed {
				return false, fmt.Errorf("VirtualMachineOperation %q failed", virtualizationEvictName)
			}
			if phase != vmopPhaseCompleted {
				return false, nil
			}

			currentNode, err := c.virtualMachineNodeName(ctx)
			if err != nil {
				return false, err
			}
			return currentNode != "" && currentNode != initialNode, nil
		},
	)
}

func (c *virtualMachineLifecycleChecker) verifyVirtualMachineMigration(ctx context.Context) error {
	availableNodes, err := c.virtualMachineClassAvailableNodes(ctx)
	if err != nil {
		return err
	}
	if len(availableNodes) < 2 {
		return nil
	}

	initialNode, err := c.virtualMachineNodeName(ctx)
	if err != nil {
		return err
	}
	if initialNode == "" {
		return fmt.Errorf("VirtualMachine %q has empty status.nodeName", virtualizationVMName)
	}

	if err := c.createVirtualMachineEvictOperation(ctx); err != nil {
		return err
	}

	return c.waitVirtualMachineMigrationCompleted(ctx, initialNode)
}

func (c *virtualMachineLifecycleChecker) waitVirtualMachineAbsent(ctx context.Context) error {
	return c.waitResourceAbsent(ctx, virtualMachineGVR, virtualizationVMName, c.waitDeletionTimeout)
}

func (c *virtualMachineLifecycleChecker) virtualMachineNodeName(ctx context.Context) (string, error) {
	obj, err := c.access.Kubernetes().Dynamic().
		Resource(virtualMachineGVR).
		Namespace(c.namespace).
		Get(ctx, virtualizationVMName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return unstructuredNestedString(obj.Object, "status", "nodeName"), nil
}

func (c *virtualMachineLifecycleChecker) virtualMachineClassAvailableNodes(ctx context.Context) ([]string, error) {
	vmClassName, err := c.virtualMachineClassName(ctx)
	if err != nil {
		return nil, err
	}

	vmClass, err := c.access.Kubernetes().Dynamic().
		Resource(virtualMachineClassGVR).
		Get(ctx, vmClassName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return unstructuredNestedStringSlice(vmClass.Object, "status", "availableNodes"), nil
}

func (c *virtualMachineLifecycleChecker) virtualMachineClassName(ctx context.Context) (string, error) {
	vm, err := c.access.Kubernetes().Dynamic().
		Resource(virtualMachineGVR).
		Namespace(c.namespace).
		Get(ctx, virtualizationVMName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if vmClassName := unstructuredNestedString(vm.Object, "spec", "virtualMachineClassName"); vmClassName != "" {
		return vmClassName, nil
	}

	defaultVMClass, err := c.defaultVirtualMachineClass(ctx)
	if err != nil {
		return "", err
	}
	if defaultVMClass == "" {
		return "", fmt.Errorf("default VirtualMachineClass not found")
	}
	return defaultVMClass, nil
}

func (c *virtualMachineLifecycleChecker) defaultVirtualMachineClass(ctx context.Context) (string, error) {
	list, err := c.access.Kubernetes().Dynamic().
		Resource(virtualMachineClassGVR).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	for i := range list.Items {
		item := &list.Items[i]
		if item.GetAnnotations()[defaultVMClassAnnotation] == "true" {
			return item.GetName(), nil
		}
	}
	return "", nil
}

func (c *virtualMachineLifecycleChecker) waitResourcePhase(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	name, phase string,
	timeout time.Duration,
) error {
	return waitForCondition(
		timeout,
		pollingInterval(timeout),
		func() (bool, error) {
			obj, err := c.access.Kubernetes().Dynamic().
				Resource(gvr).
				Namespace(c.namespace).
				Get(ctx, name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			return unstructuredNestedString(obj.Object, "status", "phase") == phase, nil
		},
	)
}

func (c *virtualMachineLifecycleChecker) waitResourceCondition(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	name, conditionType, conditionStatus string,
	timeout time.Duration,
) error {
	return waitForCondition(
		timeout,
		pollingInterval(timeout),
		func() (bool, error) {
			obj, err := c.access.Kubernetes().Dynamic().
				Resource(gvr).
				Namespace(c.namespace).
				Get(ctx, name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			return unstructuredConditionStatus(obj.Object, conditionType) == conditionStatus, nil
		},
	)
}

func (c *virtualMachineLifecycleChecker) waitResourceAbsent(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	name string,
	timeout time.Duration,
) error {
	return waitForCondition(
		timeout,
		pollingInterval(timeout),
		func() (bool, error) {
			_, err := c.access.Kubernetes().Dynamic().
				Resource(gvr).
				Namespace(c.namespace).
				Get(ctx, name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			if err != nil {
				return false, err
			}
			return false, nil
		},
	)
}

func (c *virtualMachineLifecycleChecker) virtualMachineExists(ctx context.Context) (bool, error) {
	_, err := c.access.Kubernetes().Dynamic().
		Resource(virtualMachineGVR).
		Namespace(c.namespace).
		Get(ctx, virtualizationVMName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *virtualMachineLifecycleChecker) virtualDiskExists(ctx context.Context) (bool, error) {
	_, err := c.access.Kubernetes().Dynamic().
		Resource(virtualDiskGVR).
		Namespace(c.namespace).
		Get(ctx, virtualizationDiskName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *virtualMachineLifecycleChecker) virtualImageExists(ctx context.Context) (bool, error) {
	_, err := c.access.Kubernetes().Dynamic().
		Resource(virtualImageGVR).
		Namespace(c.namespace).
		Get(ctx, c.virtualImageName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *virtualMachineLifecycleChecker) hasGarbage(ctx context.Context) (bool, error) {
	if exists, err := namespaceExists(ctx, c.access, c.namespace); err != nil {
		return false, err
	} else if exists {
		return true, nil
	}

	if exists, err := c.virtualMachineExists(ctx); err != nil {
		return false, err
	} else if exists {
		return true, nil
	}

	if exists, err := c.virtualDiskExists(ctx); err != nil {
		return false, err
	} else if exists {
		return true, nil
	}

	if exists, err := c.virtualImageExists(ctx); err != nil {
		return false, err
	} else if exists {
		return true, nil
	}

	return false, nil
}

func (c *virtualMachineLifecycleChecker) cleanup(ctx context.Context) error {
	var errs []error

	if err := c.runStep("cleanup: delete VirtualMachine", func() error {
		err := c.deleteVirtualMachine(ctx)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}); err != nil {
		errs = append(errs, fmt.Errorf("delete VirtualMachine: %w", err))
	}
	if err := c.runStep("cleanup: wait VirtualMachine deletion", func() error {
		return c.waitVirtualMachineAbsent(ctx)
	}); err != nil {
		errs = append(errs, fmt.Errorf("wait VirtualMachine deletion: %w", err))
	}
	if err := c.runStep("cleanup: delete VirtualDisk", func() error {
		err := c.deleteVirtualDisk(ctx)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}); err != nil {
		errs = append(errs, fmt.Errorf("delete VirtualDisk: %w", err))
	}
	if err := c.runStep("cleanup: wait VirtualDisk deletion", func() error {
		return c.waitVirtualDiskAbsent(ctx)
	}); err != nil {
		errs = append(errs, fmt.Errorf("wait VirtualDisk deletion: %w", err))
	}
	if err := c.runStep("cleanup: delete VirtualImage", func() error {
		err := c.deleteVirtualImage(ctx)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}); err != nil {
		errs = append(errs, fmt.Errorf("delete VirtualImage: %w", err))
	}
	if err := c.runStep("cleanup: wait VirtualImage deletion", func() error {
		return c.waitVirtualImageAbsent(ctx)
	}); err != nil {
		errs = append(errs, fmt.Errorf("wait VirtualImage deletion: %w", err))
	}
	if err := c.runStep("cleanup: delete namespace", func() error {
		err := c.deleteNamespace(ctx)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}); err != nil {
		errs = append(errs, fmt.Errorf("delete namespace: %w", err))
	}
	if err := c.runStep("cleanup: wait namespace deletion", func() error {
		return waitNamespaceNotFound(
			ctx,
			c.access,
			c.namespace,
			c.waitNamespaceDeletedTimeout,
			pollingInterval(c.waitNamespaceDeletedTimeout),
		)
	}); err != nil {
		errs = append(errs, fmt.Errorf("wait namespace deletion: %w", err))
	}

	return errors.Join(errs...)
}

func unstructuredNestedString(obj map[string]interface{}, fields ...string) string {
	value, found, err := unstructured.NestedString(obj, fields...)
	if err != nil || !found {
		return ""
	}
	return value
}

func unstructuredNestedStringSlice(obj map[string]interface{}, fields ...string) []string {
	values, found, err := unstructured.NestedStringSlice(obj, fields...)
	if err != nil || !found {
		return nil
	}
	return values
}

func unstructuredConditionStatus(obj map[string]interface{}, conditionType string) string {
	conditions, found, err := unstructured.NestedSlice(obj, "status", "conditions")
	if err != nil || !found {
		return ""
	}

	for _, condition := range conditions {
		conditionMap, ok := condition.(map[string]interface{})
		if !ok {
			continue
		}
		if unstructuredNestedString(conditionMap, "type") != conditionType {
			continue
		}
		return unstructuredNestedString(conditionMap, "status")
	}

	return ""
}

func virtualImageManifest(agentID, namespace, name, imageURL string) string {
	return fmt.Sprintf(`
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %q
    upmeter-group: extensions
    upmeter-probe: virtualization
  name: %q
  namespace: %q
spec:
  storage: ContainerRegistry
  dataSource:
    type: HTTP
    http:
      url: %q
`, agentID, name, namespace, imageURL)
}

func virtualDiskManifest(agentID, namespace, name, virtualImageName string) string {
	return fmt.Sprintf(`
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %q
    upmeter-group: extensions
    upmeter-probe: virtualization
  name: %q
  namespace: %q
spec:
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualImage
      name: %q
`, agentID, name, namespace, virtualImageName)
}

func virtualMachineManifest(agentID, namespace, name, diskName string) string {
	return fmt.Sprintf(`
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %q
    upmeter-group: extensions
    upmeter-probe: virtualization
  name: %q
  namespace: %q
spec:
  runPolicy: AlwaysOn
  cpu:
    cores: 1
    coreFraction: 5%%
  memory:
    size: 256Mi
  blockDeviceRefs:
    - kind: VirtualDisk
      name: %q
`, agentID, name, namespace, diskName)
}

func virtualMachineOperationManifest(agentID, namespace, name, vmName string) string {
	return fmt.Sprintf(`
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %q
    upmeter-group: extensions
    upmeter-probe: virtualization
  name: %q
  namespace: %q
spec:
  virtualMachineName: %q
  type: Evict
`, agentID, name, namespace, vmName)
}
