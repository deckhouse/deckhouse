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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

const (
	// VirtualizationGroupName is the upmeter group name for virtualization probes.
	VirtualizationGroupName = "virtualization"
	// VirtualizationCreationProbeName is the probe name for VM creation lifecycle.
	VirtualizationCreationProbeName = "vm-creation"
	// VirtualizationLifecycleProbeName is the probe name for VM lifecycle operations.
	VirtualizationLifecycleProbeName = "vm-lifecycle"
	// VirtualizationImageName is the VirtualImage name used by VM lifecycle probes.
	VirtualizationImageName               = "probe-image"
	virtualizationVMName                  = "probe-vm"
	virtualizationVMHTTPGuestPort         = int32(80)
	virtualizationVMNetworkPolicyName     = "probe-vm-http"
	virtualizationDiskName                = "probe-disk"
	virtualizationExtraDiskName           = "probe-extra-disk"
	virtualizationExtraDiskAttachmentName = "probe-extra-disk-attachment"
	virtualizationEvictName               = "probe-vm-evict"

	virtualizationPhaseReady   = "Ready"
	virtualizationPhaseRunning = "Running"
	vmbdaPhaseAttached         = "Attached"
	vmbdaPhaseFailed           = "Failed"
	vmopPhaseCompleted         = "Completed"
	vmopPhaseFailed            = "Failed"
	vmopTypeEvict              = "Evict"

	virtualizationConditionAgentReady = "AgentReady"

	defaultVMClassAnnotation = "virtualmachineclass.virtualization.deckhouse.io/is-default-class"

	// Baseline minimum VM sizing for the probe: functional floor regardless of VMClass policy.
	baselineVMCores        = 1
	baselineVMCoreFraction = "5%"
	baselineVMMemory       = "128Mi"
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
	virtualMachineBlockDeviceAttachmentGVR = schema.GroupVersionResource{
		Group:    "virtualization.deckhouse.io",
		Version:  "v1alpha2",
		Resource: "virtualmachineblockdeviceattachments",
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

	AgentID                 string
	Namespace               string
	ProbeName               string
	VirtualImageName        string
	VirtualImageURL         string
	VirtualMachineClassName string
	VerifyLifecycle         bool

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
		access:                            c.Access,
		preflightChecker:                  c.PreflightChecker,
		logger:                            c.Logger,
		agentID:                           fallbackString(c.AgentID, "unknown"),
		namespace:                         c.Namespace,
		probeName:                         fallbackString(c.ProbeName, VirtualizationCreationProbeName),
		virtualImageName:                  c.VirtualImageName,
		virtualImageURL:                   c.VirtualImageURL,
		configuredVirtualMachineClassName: c.VirtualMachineClassName,
		verifyLifecycle:                   c.VerifyLifecycle,

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

	agentID                           string
	namespace                         string
	probeName                         string
	virtualImageName                  string
	virtualImageURL                   string
	configuredVirtualMachineClassName string
	verifyLifecycle                   bool

	requestTimeout                     time.Duration
	waitVirtualImageTimeout            time.Duration
	waitVirtualDiskTimeout             time.Duration
	waitVirtualMachineTimeout          time.Duration
	waitVirtualMachineMigrationTimeout time.Duration
	waitDeletionTimeout                time.Duration
	waitNamespaceDeletedTimeout        time.Duration
}

type guestInventory struct {
	Disks      []guestDisk      `json:"disks"`
	NetDevices []guestNetDevice `json:"net_devices"`
}

type guestDisk struct {
	Device     string `json:"device"`
	Name       string `json:"name"`
	TotalBytes uint64 `json:"total_bytes"`
	SizeBytes  uint64 `json:"size_bytes"`
}

type guestNetDevice struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
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
	if result := c.doVirtualMachineSetup(ctx); result != nil {
		return result
	}

	if !c.verifyLifecycle {
		return nil
	}

	return c.verifyVirtualMachineLifecycle(ctx)
}

func (c *virtualMachineLifecycleChecker) doVirtualMachineSetup(ctx context.Context) check.Error {
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
				"upmeter-group": VirtualizationGroupName,
				"upmeter-probe": c.probeName,
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

	manifest := virtualImageManifest(c.agentID, c.namespace, c.probeName, c.virtualImageName, c.virtualImageURL)
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
	manifest := virtualDiskManifest(c.agentID, c.namespace, c.probeName, virtualizationDiskName, c.virtualImageName)
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

func (c *virtualMachineLifecycleChecker) createBlankVirtualDisk(ctx context.Context, name, size string) error {
	manifest := blankVirtualDiskManifest(c.agentID, c.namespace, c.probeName, name, size)
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

func (c *virtualMachineLifecycleChecker) deleteVirtualDiskByName(ctx context.Context, name string) error {
	return c.access.Kubernetes().Dynamic().
		Resource(virtualDiskGVR).
		Namespace(c.namespace).
		Delete(ctx, name, metav1.DeleteOptions{})
}

func (c *virtualMachineLifecycleChecker) waitVirtualDiskReady(ctx context.Context) error {
	return c.waitVirtualDiskReadyByName(ctx, virtualizationDiskName)
}

func (c *virtualMachineLifecycleChecker) waitVirtualDiskReadyByName(ctx context.Context, name string) error {
	return c.waitResourcePhase(
		ctx,
		virtualDiskGVR,
		name,
		virtualizationPhaseReady,
		c.waitVirtualDiskTimeout,
	)
}

func (c *virtualMachineLifecycleChecker) waitVirtualDiskAbsent(ctx context.Context) error {
	return c.waitResourceAbsent(ctx, virtualDiskGVR, virtualizationDiskName, c.waitDeletionTimeout)
}

func (c *virtualMachineLifecycleChecker) waitVirtualDiskAbsentByName(ctx context.Context, name string) error {
	return c.waitResourceAbsent(ctx, virtualDiskGVR, name, c.waitDeletionTimeout)
}

func (c *virtualMachineLifecycleChecker) resizeVirtualDisk(ctx context.Context, name, size string) error {
	obj, err := c.access.Kubernetes().Dynamic().
		Resource(virtualDiskGVR).
		Namespace(c.namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if err := unstructured.SetNestedField(obj.Object, size, "spec", "persistentVolumeClaim", "size"); err != nil {
		return err
	}
	_, err = c.access.Kubernetes().Dynamic().
		Resource(virtualDiskGVR).
		Namespace(c.namespace).
		Update(ctx, obj, metav1.UpdateOptions{})
	return err
}

func (c *virtualMachineLifecycleChecker) waitVirtualDiskCapacity(ctx context.Context, name, expectedSize string) error {
	expected, err := resource.ParseQuantity(expectedSize)
	if err != nil {
		return err
	}

	return waitForCondition(
		c.waitVirtualDiskTimeout,
		pollingInterval(c.waitVirtualDiskTimeout),
		func() (bool, error) {
			obj, err := c.access.Kubernetes().Dynamic().
				Resource(virtualDiskGVR).
				Namespace(c.namespace).
				Get(ctx, name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if unstructuredNestedString(obj.Object, "status", "phase") != virtualizationPhaseReady {
				return false, nil
			}
			capacity := unstructuredNestedString(obj.Object, "status", "capacity")
			if capacity == "" {
				return false, nil
			}
			actual, err := resource.ParseQuantity(capacity)
			if err != nil {
				return false, err
			}
			return actual.Cmp(expected) == 0, nil
		},
	)
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
	cores, coreFraction, memorySize, err := c.resolveVMSizing(ctx)
	if err != nil {
		return err
	}

	manifest := virtualMachineManifest(
		c.agentID,
		c.namespace,
		c.probeName,
		virtualizationVMName,
		virtualizationDiskName,
		c.configuredVirtualMachineClassName,
		cores,
		coreFraction,
		memorySize,
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

func (c *virtualMachineLifecycleChecker) createVirtualMachineNetworkPolicy(ctx context.Context) error {
	_, err := c.access.Kubernetes().NetworkingV1().
		NetworkPolicies(c.namespace).
		Create(ctx, virtualMachineNetworkPolicy(c.agentID, c.namespace, c.probeName), metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (c *virtualMachineLifecycleChecker) deleteVirtualMachineNetworkPolicy(ctx context.Context) error {
	return c.access.Kubernetes().NetworkingV1().
		NetworkPolicies(c.namespace).
		Delete(ctx, virtualizationVMNetworkPolicyName, metav1.DeleteOptions{})
}

func (c *virtualMachineLifecycleChecker) createVirtualMachineEvictOperation(ctx context.Context) error {
	manifest := virtualMachineOperationManifest(c.agentID, c.namespace, c.probeName, virtualizationEvictName, virtualizationVMName)
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

func (c *virtualMachineLifecycleChecker) createVirtualMachineBlockDeviceAttachment(ctx context.Context) error {
	manifest := virtualMachineBlockDeviceAttachmentManifest(
		c.agentID,
		c.namespace,
		c.probeName,
		virtualizationExtraDiskAttachmentName,
		virtualizationVMName,
		virtualizationExtraDiskName,
	)
	obj, err := decodeManifestToUnstructured(manifest)
	if err != nil {
		return err
	}
	_, err = c.access.Kubernetes().Dynamic().
		Resource(virtualMachineBlockDeviceAttachmentGVR).
		Namespace(c.namespace).
		Create(ctx, obj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (c *virtualMachineLifecycleChecker) deleteVirtualMachineBlockDeviceAttachment(ctx context.Context) error {
	return c.access.Kubernetes().Dynamic().
		Resource(virtualMachineBlockDeviceAttachmentGVR).
		Namespace(c.namespace).
		Delete(ctx, virtualizationExtraDiskAttachmentName, metav1.DeleteOptions{})
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
		string(metav1.ConditionTrue),
		c.waitVirtualMachineTimeout,
	)
}

func (c *virtualMachineLifecycleChecker) waitVirtualMachineBlockDeviceAttachmentAttached(ctx context.Context) error {
	return waitForCondition(
		c.waitVirtualMachineTimeout,
		pollingInterval(c.waitVirtualMachineTimeout),
		func() (bool, error) {
			obj, err := c.access.Kubernetes().Dynamic().
				Resource(virtualMachineBlockDeviceAttachmentGVR).
				Namespace(c.namespace).
				Get(ctx, virtualizationExtraDiskAttachmentName, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			if err != nil {
				return false, err
			}

			phase := unstructuredNestedString(obj.Object, "status", "phase")
			if phase == vmbdaPhaseFailed {
				return false, fmt.Errorf("VirtualMachineBlockDeviceAttachment %q failed", virtualizationExtraDiskAttachmentName)
			}
			return phase == vmbdaPhaseAttached, nil
		},
	)
}

func (c *virtualMachineLifecycleChecker) waitVirtualMachineBlockDeviceAttachmentAbsent(ctx context.Context) error {
	return c.waitResourceAbsent(
		ctx,
		virtualMachineBlockDeviceAttachmentGVR,
		virtualizationExtraDiskAttachmentName,
		c.waitDeletionTimeout,
	)
}

func (c *virtualMachineLifecycleChecker) waitVirtualDiskAttachedToVirtualMachine(ctx context.Context, diskName string, attached bool) error {
	return waitForCondition(
		c.waitVirtualMachineTimeout,
		pollingInterval(c.waitVirtualMachineTimeout),
		func() (bool, error) {
			vm, err := c.access.Kubernetes().Dynamic().
				Resource(virtualMachineGVR).
				Namespace(c.namespace).
				Get(ctx, virtualizationVMName, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			if err != nil {
				return false, err
			}

			currentAttached := virtualMachineHasAttachedDisk(vm.Object, diskName)
			return currentAttached == attached, nil
		},
	)
}

func (c *virtualMachineLifecycleChecker) waitVirtualMachineGuestReady(ctx context.Context) (guestInventory, error) {
	var inventory guestInventory
	err := waitForCondition(
		c.waitVirtualMachineTimeout,
		pollingInterval(c.waitVirtualMachineTimeout),
		func() (bool, error) {
			current, err := c.virtualMachineGuestInventory(ctx)
			if err != nil {
				return false, nil
			}
			if len(current.NetDevices) == 0 {
				return false, nil
			}
			inventory = current
			return true, nil
		},
	)
	return inventory, err
}

func (c *virtualMachineLifecycleChecker) waitGuestExtraDiskAttached(ctx context.Context, baseline guestInventory) error {
	return waitForCondition(
		c.waitVirtualMachineTimeout,
		pollingInterval(c.waitVirtualMachineTimeout),
		func() (bool, error) {
			inventory, err := c.virtualMachineGuestInventory(ctx)
			if err != nil {
				return false, nil
			}
			_, found := guestExtraDisk(inventory, baseline)
			return found, nil
		},
	)
}

func (c *virtualMachineLifecycleChecker) waitGuestExtraDiskSize(ctx context.Context, baseline guestInventory, expectedSize string) error {
	expected, err := resource.ParseQuantity(expectedSize)
	if err != nil {
		return err
	}

	return waitForCondition(
		c.waitVirtualMachineTimeout,
		pollingInterval(c.waitVirtualMachineTimeout),
		func() (bool, error) {
			inventory, err := c.virtualMachineGuestInventory(ctx)
			if err != nil {
				return false, nil
			}
			disk, found := guestExtraDisk(inventory, baseline)
			return found && int64(disk.Bytes()) >= expected.Value(), nil
		},
	)
}

func (c *virtualMachineLifecycleChecker) waitGuestExtraDiskDetached(ctx context.Context, baseline guestInventory) error {
	return waitForCondition(
		c.waitDeletionTimeout,
		pollingInterval(c.waitDeletionTimeout),
		func() (bool, error) {
			inventory, err := c.virtualMachineGuestInventory(ctx)
			if err != nil {
				return false, nil
			}
			_, found := guestExtraDisk(inventory, baseline)
			return !found, nil
		},
	)
}

func (c *virtualMachineLifecycleChecker) virtualMachineGuestInventory(ctx context.Context) (guestInventory, error) {
	start := time.Now()
	ip, err := c.virtualMachineIPAddress(ctx)
	if err != nil {
		c.logGuestAttempt("ip-resolve-error", "", 0, time.Since(start), err)
		return guestInventory{}, err
	}
	if ip == "" {
		err = fmt.Errorf("VirtualMachine %q has empty status.ipAddress", virtualizationVMName)
		c.logGuestAttempt("empty-ip", "", 0, time.Since(start), err)
		return guestInventory{}, err
	}

	url := fmt.Sprintf("http://%s:%d/json", ip, virtualizationVMHTTPGuestPort)

	requestCtx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, url, nil)
	if err != nil {
		c.logGuestAttempt("request-build-error", ip, 0, time.Since(start), err)
		return guestInventory{}, err
	}
	// TODO: revisit whether keep-alive can be re-enabled once the DVP/Cilium
	// behavior for VM pod IP reuse at migration is documented. Until then,
	// keep each guest inventory request on its own TCP connection: the VM pod
	// IP is reused across live migration, so a shared keep-alive connection
	// can outlive the pod it was opened to.
	req.Close = true
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.logGuestAttempt("http-error", ip, 0, time.Since(start), err)
		return guestInventory{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("guest inventory returned status %s", resp.Status)
		c.logGuestAttempt("bad-status", ip, resp.StatusCode, time.Since(start), err)
		return guestInventory{}, err
	}
	var inventory guestInventory
	if err := json.NewDecoder(resp.Body).Decode(&inventory); err != nil {
		c.logGuestAttempt("decode-error", ip, resp.StatusCode, time.Since(start), err)
		return guestInventory{}, err
	}
	c.logGuestAttempt("ok", ip, resp.StatusCode, time.Since(start), nil)
	return inventory, nil
}

// logGuestAttempt logs each guest HTTP attempt for debugging intermittent
// failures (especially after live migration). Kept lightweight: one line per
// attempt with IP, HTTP status, elapsed and error.
func (c *virtualMachineLifecycleChecker) logGuestAttempt(outcome, ip string, statusCode int, elapsed time.Duration, err error) {
	if c.logger == nil {
		return
	}
	fields := logrus.Fields{
		"outcome":     outcome,
		"ip":          ip,
		"status_code": statusCode,
		"elapsed_ms":  elapsed.Milliseconds(),
	}
	if err != nil {
		fields["error"] = err.Error()
	}
	c.logger.WithFields(fields).Info("guest inventory attempt")
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

func (c *virtualMachineLifecycleChecker) verifyVirtualMachineLifecycle(ctx context.Context) check.Error {
	if err := c.runStep("creating VirtualMachine NetworkPolicy", func() error {
		return c.createVirtualMachineNetworkPolicy(ctx)
	}); err != nil {
		return lifecycleStepError("creating VirtualMachine NetworkPolicy", err)
	}

	var baselineGuestInventory guestInventory
	if err := c.runStep("waiting for VirtualMachine guest HTTP", func() error {
		var err error
		baselineGuestInventory, err = c.waitVirtualMachineGuestReady(ctx)
		return err
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: VirtualMachine guest HTTP endpoint did not become ready")
		}
		return lifecycleStepError("waiting for VirtualMachine guest HTTP", err)
	}

	if err := c.runStep("checking VirtualMachine migration", func() error {
		return c.verifyVirtualMachineMigration(ctx)
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: VirtualMachine migration did not complete")
		}
		return lifecycleStepError("checking VirtualMachine migration", err)
	}

	if err := c.runStep("checking VirtualMachine guest HTTP after migration", func() error {
		_, err := c.waitVirtualMachineGuestReady(ctx)
		return err
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: VirtualMachine guest HTTP endpoint did not recover after migration")
		}
		return lifecycleStepError("checking VirtualMachine guest HTTP after migration", err)
	}

	if err := c.runStep("creating extra VirtualDisk", func() error {
		return c.createBlankVirtualDisk(ctx, virtualizationExtraDiskName, "50Mi")
	}); err != nil {
		return lifecycleStepError("creating extra VirtualDisk", err)
	}

	if err := c.runStep("attaching extra VirtualDisk", func() error {
		return c.createVirtualMachineBlockDeviceAttachment(ctx)
	}); err != nil {
		return lifecycleStepError("attaching extra VirtualDisk", err)
	}

	if err := c.runStep("waiting for extra VirtualDisk attachment", func() error {
		return c.waitVirtualMachineBlockDeviceAttachmentAttached(ctx)
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: extra VirtualDisk attachment did not become Attached")
		}
		return lifecycleStepError("waiting for extra VirtualDisk attachment", err)
	}

	if err := c.runStep("waiting for extra VirtualDisk", func() error {
		return c.waitVirtualDiskReadyByName(ctx, virtualizationExtraDiskName)
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: extra VirtualDisk did not become Ready")
		}
		return lifecycleStepError("waiting for extra VirtualDisk", err)
	}

	if err := c.runStep("checking extra VirtualDisk in VM status", func() error {
		return c.waitVirtualDiskAttachedToVirtualMachine(ctx, virtualizationExtraDiskName, true)
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: extra VirtualDisk did not appear attached in VirtualMachine status")
		}
		return lifecycleStepError("checking extra VirtualDisk in VM status", err)
	}

	if err := c.runStep("checking extra VirtualDisk in guest", func() error {
		return c.waitGuestExtraDiskAttached(ctx, baselineGuestInventory)
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: extra VirtualDisk did not appear in guest inventory")
		}
		return lifecycleStepError("checking extra VirtualDisk in guest", err)
	}

	if err := c.runStep("resizing extra VirtualDisk", func() error {
		return c.resizeVirtualDisk(ctx, virtualizationExtraDiskName, "100Mi")
	}); err != nil {
		return lifecycleStepError("resizing extra VirtualDisk", err)
	}

	if err := c.runStep("waiting for extra VirtualDisk resize", func() error {
		return c.waitVirtualDiskCapacity(ctx, virtualizationExtraDiskName, "100Mi")
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: extra VirtualDisk capacity did not become 100Mi")
		}
		return lifecycleStepError("waiting for extra VirtualDisk resize", err)
	}

	if err := c.runStep("checking extra VirtualDisk resize in guest", func() error {
		return c.waitGuestExtraDiskSize(ctx, baselineGuestInventory, "100Mi")
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: extra VirtualDisk capacity did not become 100Mi in guest inventory")
		}
		return lifecycleStepError("checking extra VirtualDisk resize in guest", err)
	}

	if err := c.runStep("checking VirtualMachine after extra VirtualDisk resize", func() error {
		if err := c.waitVirtualMachineRunning(ctx); err != nil {
			return err
		}
		if err := c.waitVirtualMachineAgentReady(ctx); err != nil {
			return err
		}
		return c.waitVirtualMachineBlockDeviceAttachmentAttached(ctx)
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: VirtualMachine did not stay ready after extra VirtualDisk resize")
		}
		return lifecycleStepError("checking VirtualMachine after extra VirtualDisk resize", err)
	}

	if err := c.runStep("detaching extra VirtualDisk", func() error {
		return c.deleteVirtualMachineBlockDeviceAttachment(ctx)
	}); err != nil {
		return lifecycleStepError("detaching extra VirtualDisk", err)
	}

	if err := c.runStep("waiting for extra VirtualDisk detach", func() error {
		return c.waitVirtualMachineBlockDeviceAttachmentAbsent(ctx)
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: extra VirtualDisk attachment was not deleted")
		}
		return lifecycleStepError("waiting for extra VirtualDisk detach", err)
	}

	if err := c.runStep("checking extra VirtualDisk detached from VM status", func() error {
		return c.waitVirtualDiskAttachedToVirtualMachine(ctx, virtualizationExtraDiskName, false)
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: extra VirtualDisk still appears attached in VirtualMachine status")
		}
		return lifecycleStepError("checking extra VirtualDisk detached from VM status", err)
	}

	if err := c.runStep("checking extra VirtualDisk detached from guest", func() error {
		return c.waitGuestExtraDiskDetached(ctx, baselineGuestInventory)
	}); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: extra VirtualDisk still appears in guest inventory")
		}
		return lifecycleStepError("checking extra VirtualDisk detached from guest", err)
	}

	return nil
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

func (c *virtualMachineLifecycleChecker) virtualMachineIPAddress(ctx context.Context) (string, error) {
	obj, err := c.access.Kubernetes().Dynamic().
		Resource(virtualMachineGVR).
		Namespace(c.namespace).
		Get(ctx, virtualizationVMName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return unstructuredNestedString(obj.Object, "status", "ipAddress"), nil
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
	if c.configuredVirtualMachineClassName != "" {
		return c.configuredVirtualMachineClassName, nil
	}

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

// resolveVMSizing computes the minimal VM cpu/memory sizing that satisfies the
// VMClass sizing policy (if any), never dropping below the probe's functional
// baseline (1 core, 5%, 128Mi).
//
// If the VMClass has no sizing policies, or no class is resolvable, the baseline
// is returned as-is.
func (c *virtualMachineLifecycleChecker) resolveVMSizing(ctx context.Context) (int, string, string, error) {
	cores, coreFraction, memorySize := baselineVMCores, baselineVMCoreFraction, baselineVMMemory

	vmClassName, err := c.virtualMachineClassName(ctx)
	if err != nil {
		// No resolvable class: keep baseline. The VM will fail later if a class is
		// actually required, but the probe must not block on sizing resolution here.
		return cores, coreFraction, memorySize, nil
	}

	vmClass, err := c.access.Kubernetes().Dynamic().
		Resource(virtualMachineClassGVR).
		Get(ctx, vmClassName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return cores, coreFraction, memorySize, nil
		}
		return 0, "", "", err
	}

	policies, found, err := unstructured.NestedSlice(vmClass.Object, "spec", "sizingPolicies")
	if err != nil || !found || len(policies) == 0 {
		return cores, coreFraction, memorySize, nil
	}

	policy := pickMinimalSizingPolicy(policies)
	if policy == nil {
		return cores, coreFraction, memorySize, nil
	}

	cores = resolvePolicyCores(policy)
	coreFraction = resolvePolicyCoreFraction(policy)
	memorySize = resolvePolicyMemory(policy, cores)
	return cores, coreFraction, memorySize, nil
}

// pickMinimalSizingPolicy selects the sizing policy that admits the smallest VM:
// prefer a policy whose range includes 1 core; otherwise pick the one with the
// smallest cores.min.
func pickMinimalSizingPolicy(policies []interface{}) map[string]interface{} {
	var fallback map[string]interface{}
	var fallbackMin int
	for _, p := range policies {
		policy, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		min := int(unstructuredNestedInt64(policy, "cores", "min"))
		max := int(unstructuredNestedInt64(policy, "cores", "max"))
		if min < 1 {
			min = 1
		}
		if min <= 1 && 1 <= max {
			return policy
		}
		if fallback == nil || min < fallbackMin {
			fallback = policy
			fallbackMin = min
		}
	}
	return fallback
}

func resolvePolicyCores(policy map[string]interface{}) int {
	min := int(unstructuredNestedInt64(policy, "cores", "min"))
	max := int(unstructuredNestedInt64(policy, "cores", "max"))
	if min < 1 {
		min = 1
	}
	// Prefer 1 core if the range admits it, otherwise the policy minimum.
	if 1 >= min && 1 <= max {
		return 1
	}
	if min >= 1 && min <= max {
		return min
	}
	return baselineVMCores
}

// resolvePolicyCoreFraction picks the smallest allowed core fraction not below
// the 5% baseline. Fraction values in v1alpha3 are percentage strings ("5%").
func resolvePolicyCoreFraction(policy map[string]interface{}) string {
	fractions, _, _ := unstructured.NestedStringSlice(policy, "coreFractions")
	const baseline = 5
	best := 0
	for _, raw := range fractions {
		v := parseFractionPercent(raw)
		if v <= 0 {
			continue
		}
		if v < baseline {
			continue
		}
		if best == 0 || v < best {
			best = v
		}
	}
	if best > 0 {
		return fmt.Sprintf("%d%%", best)
	}
	// No allowed fraction >= baseline: fall back to the policy default if any.
	if def := unstructuredNestedString(policy, "defaultCoreFraction"); def != "" {
		return def
	}
	return baselineVMCoreFraction
}

// resolvePolicyMemory computes the smallest memory size that satisfies the
// policy (min, step quantization, per-core floor) and is not below the baseline.
func resolvePolicyMemory(policy map[string]interface{}, cores int) string {
	baseline := resource.MustParse(baselineVMMemory)

	mem, _, _ := unstructured.NestedMap(policy, "memory")
	if mem == nil {
		return baselineVMMemory
	}

	var minVal resource.Quantity
	if minStr := unstructuredNestedString(mem, "min"); minStr != "" {
		if q, err := resource.ParseQuantity(minStr); err == nil {
			minVal = q
		}
	}

	// If per-core bounds are set, the effective floor is perCore.min * cores.
	if perCore, _, _ := unstructured.NestedMap(mem, "perCore"); perCore != nil {
		if pcMinStr := unstructuredNestedString(perCore, "min"); pcMinStr != "" {
			if pcMin, err := resource.ParseQuantity(pcMinStr); err == nil {
				perCoreFloor := pcMin.DeepCopy()
				for i := 1; i < cores; i++ {
					perCoreFloor.Add(pcMin)
				}
				if perCoreFloor.Cmp(minVal) == 1 {
					minVal = perCoreFloor
				}
			}
		}
	}

	chosen := baseline
	if !minVal.IsZero() && minVal.Cmp(baseline) == 1 {
		chosen = minVal
	}

	// Round up to the step grid starting from min.
	if stepStr := unstructuredNestedString(mem, "step"); stepStr != "" {
		if step, err := resource.ParseQuantity(stepStr); err == nil && !step.IsZero() {
			start := minVal
			if start.IsZero() {
				start = baseline
			}
			for start.Cmp(chosen) == -1 {
				start.Add(step)
			}
			chosen = start
		}
	}

	// Clamp to max if present.
	if maxStr := unstructuredNestedString(mem, "max"); maxStr != "" {
		if maxQ, err := resource.ParseQuantity(maxStr); err == nil && chosen.Cmp(maxQ) == 1 {
			chosen = maxQ
		}
	}

	return chosen.String()
}

func parseFractionPercent(s string) int {
	s = strings.TrimSuffix(s, "%")
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
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

	if err := c.runStep("cleanup: delete VirtualMachine NetworkPolicy", func() error {
		err := c.deleteVirtualMachineNetworkPolicy(ctx)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}); err != nil {
		errs = append(errs, fmt.Errorf("delete VirtualMachine NetworkPolicy: %w", err))
	}
	if err := c.runStep("cleanup: delete VirtualMachineBlockDeviceAttachment", func() error {
		err := c.deleteVirtualMachineBlockDeviceAttachment(ctx)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}); err != nil {
		errs = append(errs, fmt.Errorf("delete VirtualMachineBlockDeviceAttachment: %w", err))
	}
	if err := c.runStep("cleanup: wait VirtualMachineBlockDeviceAttachment deletion", func() error {
		return c.waitVirtualMachineBlockDeviceAttachmentAbsent(ctx)
	}); err != nil {
		errs = append(errs, fmt.Errorf("wait VirtualMachineBlockDeviceAttachment deletion: %w", err))
	}
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
	if err := c.runStep("cleanup: delete extra VirtualDisk", func() error {
		err := c.deleteVirtualDiskByName(ctx, virtualizationExtraDiskName)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}); err != nil {
		errs = append(errs, fmt.Errorf("delete extra VirtualDisk: %w", err))
	}
	if err := c.runStep("cleanup: wait extra VirtualDisk deletion", func() error {
		return c.waitVirtualDiskAbsentByName(ctx, virtualizationExtraDiskName)
	}); err != nil {
		errs = append(errs, fmt.Errorf("wait extra VirtualDisk deletion: %w", err))
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

func unstructuredNestedInt64(obj map[string]interface{}, fields ...string) int64 {
	value, found, err := unstructured.NestedInt64(obj, fields...)
	if err != nil || !found {
		return 0
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

func virtualMachineHasAttachedDisk(obj map[string]interface{}, diskName string) bool {
	blockDeviceRefs, found, err := unstructured.NestedSlice(obj, "status", "blockDeviceRefs")
	if err != nil || !found {
		return false
	}

	for _, item := range blockDeviceRefs {
		ref, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if unstructuredNestedString(ref, "kind") != "VirtualDisk" {
			continue
		}
		if unstructuredNestedString(ref, "name") != diskName {
			continue
		}
		attached, found, err := unstructured.NestedBool(ref, "attached")
		return err == nil && found && attached
	}

	return false
}

func (d guestDisk) ID() string {
	if d.Device != "" {
		return d.Device
	}
	return d.Name
}

func (d guestDisk) Bytes() uint64 {
	if d.SizeBytes != 0 {
		return d.SizeBytes
	}
	return d.TotalBytes
}

func guestExtraDisk(inventory, baseline guestInventory) (guestDisk, bool) {
	baselineDisks := make(map[string]struct{}, len(baseline.Disks))
	for _, disk := range baseline.Disks {
		if id := disk.ID(); id != "" {
			baselineDisks[id] = struct{}{}
		}
	}

	for _, disk := range inventory.Disks {
		id := disk.ID()
		if id == "" {
			continue
		}
		if _, ok := baselineDisks[id]; !ok {
			return disk, true
		}
	}
	return guestDisk{}, false
}

func ptrTo[T any](value T) *T {
	return &value
}

func virtualImageManifest(agentID, namespace, probeName, name, imageURL string) string {
	return fmt.Sprintf(`
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %q
    upmeter-group: %q
    upmeter-probe: %q
  name: %q
  namespace: %q
spec:
  storage: ContainerRegistry
  dataSource:
    type: HTTP
    http:
      url: %q
`, agentID, VirtualizationGroupName, probeName, name, namespace, imageURL)
}

func virtualDiskManifest(agentID, namespace, probeName, name, virtualImageName string) string {
	return fmt.Sprintf(`
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %q
    upmeter-group: %q
    upmeter-probe: %q
  name: %q
  namespace: %q
spec:
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualImage
      name: %q
`, agentID, VirtualizationGroupName, probeName, name, namespace, virtualImageName)
}

func blankVirtualDiskManifest(agentID, namespace, probeName, name, size string) string {
	return fmt.Sprintf(`
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %q
    upmeter-group: %q
    upmeter-probe: %q
  name: %q
  namespace: %q
spec:
  persistentVolumeClaim:
    size: %q
`, agentID, VirtualizationGroupName, probeName, name, namespace, size)
}

func virtualMachineNetworkPolicy(agentID, namespace, probeName string) *netv1.NetworkPolicy {
	return &netv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: "networking.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtualizationVMNetworkPolicyName,
			Namespace: namespace,
			Labels: map[string]string{
				"heritage":      "upmeter",
				agentLabelKey:   agentID,
				"upmeter-group": VirtualizationGroupName,
				"upmeter-probe": probeName,
			},
		},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []netv1.PolicyType{
				netv1.PolicyTypeIngress,
			},
			Ingress: []netv1.NetworkPolicyIngressRule{
				{
					From: []netv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "d8-upmeter",
								},
							},
						},
					},
					Ports: []netv1.NetworkPolicyPort{
						{
							Protocol: ptrTo(v1.ProtocolTCP),
							Port:     ptrTo(intstr.FromInt(int(virtualizationVMHTTPGuestPort))),
						},
					},
				},
			},
		},
	}
}

func virtualMachineManifest(agentID, namespace, probeName, name, diskName, virtualMachineClassName string, cores int, coreFraction, memorySize string) string {
	virtualMachineClassSpec := ""
	if virtualMachineClassName != "" {
		virtualMachineClassSpec = fmt.Sprintf("  virtualMachineClassName: %q\n", virtualMachineClassName)
	}

	return fmt.Sprintf(`
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %q
    upmeter-group: %q
    upmeter-probe: %q
  name: %q
  namespace: %q
spec:
  runPolicy: AlwaysOn
%s  cpu:
    cores: %d
    coreFraction: %s
  memory:
    size: %s
  blockDeviceRefs:
    - kind: VirtualDisk
      name: %q
`, agentID, VirtualizationGroupName, probeName, name, namespace, virtualMachineClassSpec, cores, coreFraction, memorySize, diskName)
}

func virtualMachineOperationManifest(agentID, namespace, probeName, name, vmName string) string {
	return fmt.Sprintf(`
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %q
    upmeter-group: %q
    upmeter-probe: %q
  name: %q
  namespace: %q
spec:
  virtualMachineName: %q
  type: %q
`, agentID, VirtualizationGroupName, probeName, name, namespace, vmName, vmopTypeEvict)
}

func virtualMachineBlockDeviceAttachmentManifest(agentID, namespace, probeName, name, vmName, diskName string) string {
	return fmt.Sprintf(`
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineBlockDeviceAttachment
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %q
    upmeter-group: %q
    upmeter-probe: %q
  name: %q
  namespace: %q
spec:
  blockDeviceRef:
    kind: VirtualDisk
    name: %q
  virtualMachineName: %q
`, agentID, VirtualizationGroupName, probeName, name, namespace, diskName, vmName)
}
