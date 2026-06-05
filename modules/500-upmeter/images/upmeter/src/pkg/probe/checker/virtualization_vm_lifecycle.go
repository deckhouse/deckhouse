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

	virtualizationPhaseReady   = "Ready"
	virtualizationPhaseRunning = "Running"
)

var (
	clusterVirtualImageGVR = schema.GroupVersionResource{
		Group:    "virtualization.deckhouse.io",
		Version:  "v1alpha2",
		Resource: "clustervirtualimages",
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
)

// VirtualMachineLifecycle is a checker constructor and configurator.
type VirtualMachineLifecycle struct {
	Access           kubernetes.Access
	PreflightChecker check.Checker

	AgentID          string
	Namespace        string
	ClusterImageName string
	ClusterImageURL  string
	VMClassName      string

	RequestTimeout            time.Duration
	WaitClusterImageTimeout   time.Duration
	WaitVirtualDiskTimeout    time.Duration
	WaitVirtualMachineTimeout time.Duration
	WaitDeletionTimeout       time.Duration
	WaitNamespaceDeletedTimeout time.Duration
	Timeout                   time.Duration
}

func (c VirtualMachineLifecycle) Checker() check.Checker {
	checker := &virtualMachineLifecycleChecker{
		access:           c.Access,
		preflightChecker: c.PreflightChecker,
		agentID:          fallbackString(c.AgentID, "unknown"),
		namespace:        c.Namespace,
		clusterImageName: c.ClusterImageName,
		clusterImageURL:  c.ClusterImageURL,
		vmClassName:      fallbackString(c.VMClassName, "generic"),

		requestTimeout:              fallbackDuration(c.RequestTimeout, 5*time.Second),
		waitClusterImageTimeout:     fallbackDuration(c.WaitClusterImageTimeout, 15*time.Minute),
		waitVirtualDiskTimeout:      fallbackDuration(c.WaitVirtualDiskTimeout, 3*time.Minute),
		waitVirtualMachineTimeout:   fallbackDuration(c.WaitVirtualMachineTimeout, 5*time.Minute),
		waitDeletionTimeout:         fallbackDuration(c.WaitDeletionTimeout, 2*time.Minute),
		waitNamespaceDeletedTimeout: fallbackDuration(c.WaitNamespaceDeletedTimeout, 2*time.Minute),
	}

	return withTimeout(checker, fallbackDuration(c.Timeout, 25*time.Minute))
}

type virtualMachineLifecycleChecker struct {
	access           kubernetes.Access
	preflightChecker check.Checker

	agentID          string
	namespace        string
	clusterImageName string
	clusterImageURL  string
	vmClassName      string

	requestTimeout              time.Duration
	waitClusterImageTimeout     time.Duration
	waitVirtualDiskTimeout      time.Duration
	waitVirtualMachineTimeout   time.Duration
	waitDeletionTimeout         time.Duration
	waitNamespaceDeletedTimeout time.Duration
}

func (c *virtualMachineLifecycleChecker) Check() check.Error {
	ctx := context.Background()

	if err := c.preflight(); err != nil {
		return err
	}

	hasGarbage, err := c.hasGarbage(ctx)
	if err != nil {
		return check.ErrUnknown("checking garbage: %v", err)
	}
	if hasGarbage {
		if cleanupErr := c.cleanup(ctx); cleanupErr != nil {
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
	if err := c.ensureClusterVirtualImageReady(ctx); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail(
				"verification: ClusterVirtualImage %q is not Ready",
				c.clusterImageName,
			)
		}
		return lifecycleStepError("ensuring ClusterVirtualImage", err)
	}

	if err := c.createNamespace(ctx); err != nil {
		return lifecycleStepError("creating namespace", err)
	}

	if err := c.createVirtualDisk(ctx); err != nil {
		return lifecycleStepError("creating VirtualDisk", err)
	}

	if err := c.waitVirtualDiskReady(ctx); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: VirtualDisk did not become Ready")
		}
		return lifecycleStepError("waiting for VirtualDisk", err)
	}

	if err := c.createVirtualMachine(ctx); err != nil {
		return lifecycleStepError("creating VirtualMachine", err)
	}

	if err := c.waitVirtualMachineRunning(ctx); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: VirtualMachine did not reach Running phase")
		}
		return lifecycleStepError("waiting for VirtualMachine Running", err)
	}

	if err := c.deleteVirtualMachine(ctx); err != nil && !apierrors.IsNotFound(err) {
		return lifecycleStepError("deleting VirtualMachine", err)
	}

	if err := c.waitVirtualMachineAbsent(ctx); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: VirtualMachine was not deleted")
		}
		return lifecycleStepError("waiting for VirtualMachine deletion", err)
	}

	if err := c.deleteVirtualDisk(ctx); err != nil && !apierrors.IsNotFound(err) {
		return lifecycleStepError("deleting VirtualDisk", err)
	}

	if err := c.waitVirtualDiskAbsent(ctx); err != nil {
		if errors.Is(err, errConditionTimeout) {
			return check.ErrFail("verification: VirtualDisk was not deleted")
		}
		return lifecycleStepError("waiting for VirtualDisk deletion", err)
	}

	return nil
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

func (c *virtualMachineLifecycleChecker) ensureClusterVirtualImageReady(ctx context.Context) error {
	if err := c.createClusterVirtualImageIfMissing(ctx); err != nil {
		return err
	}
	return c.waitClusterVirtualImageReady(ctx)
}

func (c *virtualMachineLifecycleChecker) createClusterVirtualImageIfMissing(ctx context.Context) error {
	_, err := c.access.Kubernetes().Dynamic().
		Resource(clusterVirtualImageGVR).
		Get(ctx, c.clusterImageName, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	if c.clusterImageURL == "" {
		return fmt.Errorf("ClusterVirtualImage %q not found and clusterImageURL is not configured", c.clusterImageName)
	}

	manifest := clusterVirtualImageManifest(c.agentID, c.clusterImageName, c.clusterImageURL)
	obj, err := decodeManifestToUnstructured(manifest)
	if err != nil {
		return err
	}
	_, err = c.access.Kubernetes().Dynamic().
		Resource(clusterVirtualImageGVR).
		Create(ctx, obj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (c *virtualMachineLifecycleChecker) waitClusterVirtualImageReady(ctx context.Context) error {
	return waitForCondition(
		c.waitClusterImageTimeout,
		pollingInterval(c.waitClusterImageTimeout),
		func() (bool, error) {
			obj, err := c.access.Kubernetes().Dynamic().
				Resource(clusterVirtualImageGVR).
				Get(ctx, c.clusterImageName, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return false, fmt.Errorf("ClusterVirtualImage %q not found", c.clusterImageName)
			}
			if err != nil {
				return false, err
			}
			return unstructuredNestedString(obj.Object, "status", "phase") == virtualizationPhaseReady, nil
		},
	)
}

func (c *virtualMachineLifecycleChecker) createVirtualDisk(ctx context.Context) error {
	manifest := virtualDiskManifest(c.agentID, c.namespace, virtualizationDiskName, c.clusterImageName)
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

func (c *virtualMachineLifecycleChecker) createVirtualMachine(ctx context.Context) error {
	manifest := virtualMachineManifest(
		c.agentID,
		c.namespace,
		virtualizationVMName,
		virtualizationDiskName,
		c.vmClassName,
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

func (c *virtualMachineLifecycleChecker) waitVirtualMachineAbsent(ctx context.Context) error {
	return c.waitResourceAbsent(ctx, virtualMachineGVR, virtualizationVMName, c.waitDeletionTimeout)
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

	return false, nil
}

func (c *virtualMachineLifecycleChecker) cleanup(ctx context.Context) error {
	var errs []error

	if err := c.deleteVirtualMachine(ctx); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("delete VirtualMachine: %w", err))
	}
	if err := c.waitVirtualMachineAbsent(ctx); err != nil {
		errs = append(errs, fmt.Errorf("wait VirtualMachine deletion: %w", err))
	}
	if err := c.deleteVirtualDisk(ctx); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("delete VirtualDisk: %w", err))
	}
	if err := c.waitVirtualDiskAbsent(ctx); err != nil {
		errs = append(errs, fmt.Errorf("wait VirtualDisk deletion: %w", err))
	}
	if err := c.deleteNamespace(ctx); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("delete namespace: %w", err))
	}
	if err := waitNamespaceNotFound(
		ctx,
		c.access,
		c.namespace,
		c.waitNamespaceDeletedTimeout,
		pollingInterval(c.waitNamespaceDeletedTimeout),
	); err != nil {
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

func clusterVirtualImageManifest(agentID, name, imageURL string) string {
	return fmt.Sprintf(`
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: ClusterVirtualImage
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: %q
    upmeter-group: extensions
    upmeter-probe: virtualization
  name: %q
spec:
  storage: ContainerRegistry
  dataSource:
    type: HTTP
    http:
      url: %q
`, agentID, name, imageURL)
}

func virtualDiskManifest(agentID, namespace, name, clusterImageName string) string {
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
      kind: ClusterVirtualImage
      name: %q
`, agentID, name, namespace, clusterImageName)
}

func virtualMachineManifest(agentID, namespace, name, diskName, vmClassName string) string {
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
  virtualMachineClassName: %q
  runPolicy: AlwaysOn
  cpu:
    cores: 1
  memory:
    size: 1Gi
  blockDeviceRefs:
    - kind: VirtualDisk
      name: %q
`, agentID, name, namespace, vmClassName, diskName)
}
