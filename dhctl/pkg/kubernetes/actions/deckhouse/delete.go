// Copyright 2021 Flant JSC
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

package deckhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

const (
	deckhouseDeploymentNamespace = "d8-system"
	deckhouseDeploymentName      = "deckhouse"
)

var d8storageConfig = []schema.GroupVersionResource{
	{
		Group:    "storage.deckhouse.io",
		Version:  "v1alpha1",
		Resource: "localstorageclasses",
	},
	{
		Group:    "storage.deckhouse.io",
		Version:  "v1alpha1",
		Resource: "replicatedstorageclasses",
	},
	{
		Group:    "storage.deckhouse.io",
		Version:  "v1alpha1",
		Resource: "nfsstorageclasses",
	},
	{
		Group:    "storage.deckhouse.io",
		Version:  "v1alpha1",
		Resource: "cephstorageclasses",
	},
}

func DeleteDeckhouseDeployment(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Delete Deckhouse", 45, 5*time.Second).WithShowError(false).RunContext(ctx, func() error {
		foregroundPolicy := metav1.DeletePropagationForeground
		err := kubeCl.AppsV1().Deployments(deckhouseDeploymentNamespace).Delete(ctx, deckhouseDeploymentName, metav1.DeleteOptions{PropagationPolicy: &foregroundPolicy})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		return nil
	})
}

func DeletePDBs(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Delete pdbs", 45, 5*time.Second).WithShowError(false).RunContext(ctx, func() error {
		foregroundPolicy := metav1.DeletePropagationForeground
		namespaces, err := kubeCl.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, ns := range namespaces.Items {
			pdbs, err := kubeCl.PolicyV1().PodDisruptionBudgets(ns.Name).List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}

			for _, pdb := range pdbs.Items {
				err := kubeCl.PolicyV1().PodDisruptionBudgets(ns.Name).Delete(ctx, pdb.Name, metav1.DeleteOptions{PropagationPolicy: &foregroundPolicy})
				if err != nil {
					return err
				}
			}
		}

		return nil
	})
}

func ListD8StorageResources(ctx context.Context, kubeCl *client.KubernetesClient, cr schema.GroupVersionResource) (*unstructured.UnstructuredList, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	storageCR, err := kubeCl.Dynamic().Resource(cr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return storageCR, err
}

func DeleteD8StorageResources(ctx context.Context, kubeCl *client.KubernetesClient, obj unstructured.Unstructured, cr schema.GroupVersionResource) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err := kubeCl.Dynamic().Resource(cr).Namespace(obj.GetNamespace()).Delete(ctx, obj.GetName(), metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("delete %s %s: %v", cr, obj.GetName(), err)
	}
	log.InfoF("%s/%s\n", obj.GetKind(), obj.GetName())
	return nil
}

func DeleteAllD8StorageResources(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Delete Deckhouse Storage CRs", 45, 5*time.Second).WithShowError(false).RunContext(ctx, func() error {
		for _, cr := range d8storageConfig {
			storageCRs, err := ListD8StorageResources(ctx, kubeCl, cr)
			if err != nil {
				if errors.IsNotFound(err) {
					log.InfoF("Resources kind of %s not found, skipping...\n", cr)
					continue
				}
				return fmt.Errorf("get %s: %v", cr, err)
			}
			for _, obj := range storageCRs.Items {
				err = DeleteD8StorageResources(ctx, kubeCl, obj, cr)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func DeleteStorageClasses(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Delete StorageClasses", 45, 5*time.Second).WithShowError(false).RunContext(ctx, func() error {
		return kubeCl.StorageV1().StorageClasses().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
	})
}

func DeletePods(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Delete Pods", 45, 5*time.Second).WithShowError(false).RunContext(ctx, func() error {
		pods, err := kubeCl.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, pod := range pods.Items {
			// We have to delete only pods with pvc to trigger pv/pvc deletion
			if len(pod.Spec.Volumes) == 0 {
				continue
			}

			podWithPersistentVolumeClaim := false
			for _, volume := range pod.Spec.Volumes {
				if volume.PersistentVolumeClaim != nil {
					podWithPersistentVolumeClaim = true
					break
				}
			}

			if !podWithPersistentVolumeClaim {
				continue
			}

			err := kubeCl.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
			if err != nil {
				log.ErrorLn(err.Error())
			} else {
				log.InfoF("%s/%s\n", pod.Namespace, pod.Name)
			}
		}

		return nil
	})
}

func DeleteServices(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Delete Services", 45, 5*time.Second).WithShowError(false).RunContext(ctx, func() error {
		allServices, err := kubeCl.CoreV1().Services(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, service := range allServices.Items {
			if service.Spec.Type != v1.ServiceTypeLoadBalancer {
				continue
			}

			err := kubeCl.CoreV1().Services(service.Namespace).Delete(ctx, service.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
			log.InfoF("%s/%s\n", service.Namespace, service.Name)
		}
		return nil
	})
}

func DeletePVC(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Delete PersistentVolumeClaims", 45, 5*time.Second).WithShowError(false).RunContext(ctx, func() error {
		volumeClaims, err := kubeCl.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, claim := range volumeClaims.Items {
			err := kubeCl.CoreV1().PersistentVolumeClaims(claim.Namespace).Delete(ctx, claim.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
			log.InfoF("%s/%s\n", claim.Namespace, claim.Name)
		}
		return nil
	})
}

func WaitForDeckhouseDeploymentDeletion(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Wait for Deckhouse Deployment deletion", 30, 5*time.Second).WithShowError(false).RunContext(ctx, func() error {
		_, err := kubeCl.AppsV1().Deployments(deckhouseDeploymentNamespace).Get(ctx, deckhouseDeploymentName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			log.InfoLn("Deckhouse Deployment and its dependents are removed")
			return nil
		}

		errStr := "Deckhouse Deployment and its dependents are not removed from the cluster yet"
		if err != nil {
			errStr = fmt.Sprintf("Error during waiting, err: %v", err)
		}
		//goland:noinspection GoErrorStringFormat
		return fmt.Errorf("%s", errStr)
	})
}

func WaitForServicesDeletion(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Wait for Services deletion", 45, 15*time.Second).WithShowError(false).RunContext(ctx, func() error {
		resources, err := kubeCl.CoreV1().Services(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		var filteredResources []v1.Service
		for _, resource := range resources.Items {
			if resource.Spec.Type == v1.ServiceTypeLoadBalancer {
				filteredResources = append(filteredResources, resource)
			}
		}

		count := len(filteredResources)
		if count != 0 {
			builder := strings.Builder{}
			for _, item := range filteredResources {
				builder.WriteString(fmt.Sprintf("\t\t%s/%s\n", item.Namespace, item.Name))
			}
			return fmt.Errorf("%d Services left in the cluster\n%s", count, strings.TrimSuffix(builder.String(), "\n"))
		}
		log.InfoLn("All Services with type LoadBalancer are deleted from the cluster")
		return nil
	})
}

func WaitForPVDeletion(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Wait for PersistentVolumes deletion", 45, 15*time.Second).WithShowError(false).RunContext(ctx, func() error {
		resources, err := kubeCl.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		// Skip PV's provided manually or with reclaimPolicy other than Delete
		annotationKey := "pv.kubernetes.io/provisioned-by"
		var filteredResources []v1.PersistentVolume
		var skipPVs []v1.PersistentVolume
		for _, resource := range resources.Items {
			if _, exists := resource.Annotations[annotationKey]; !exists || resource.Spec.PersistentVolumeReclaimPolicy != v1.PersistentVolumeReclaimDelete {
				skipPVs = append(skipPVs, resource)
			} else {
				filteredResources = append(filteredResources, resource)
			}
		}

		skipPVsCount := len(skipPVs)
		if skipPVsCount != 0 {
			skipPVsInfo := strings.Builder{}
			for _, item := range skipPVs {
				skipPVsInfo.WriteString(fmt.Sprintf("\t\t%s | %s\n", item.Name, item.Status.Phase))
			}
			log.InfoF("%d PersistentVolumes provided manually or with reclaimPolicy other than Delete was skipped.\n%s\n", skipPVsCount, strings.TrimSuffix(skipPVsInfo.String(), "\n"))
		}

		count := len(filteredResources)
		if count != 0 {
			remainingPVs := strings.Builder{}
			for _, item := range filteredResources {
				remainingPVs.WriteString(fmt.Sprintf("\t\t%s | %s\n", item.Name, item.Status.Phase))
			}
			return fmt.Errorf("%d PersistentVolumes left in the cluster\n%s", count, strings.TrimSuffix(remainingPVs.String(), "\n"))
		}
		log.InfoLn("All PersistentVolumes are deleted from the cluster")
		return nil
	})
}

func WaitForPVCDeletion(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Wait for PersistentVolumeClaims deletion", 45, 15*time.Second).WithShowError(false).RunContext(ctx, func() error {
		resources, err := kubeCl.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		// Pending PVCs have no attached PVs, we have to skip them
		var filteredResources []v1.PersistentVolumeClaim
		for _, resource := range resources.Items {
			if resource.Status.Phase != v1.ClaimPending {
				filteredResources = append(filteredResources, resource)
			}
		}

		count := len(filteredResources)
		if count != 0 {
			builder := strings.Builder{}
			for _, item := range resources.Items {
				builder.WriteString(fmt.Sprintf("\t\t%s | %s\n", item.Name, item.Status.Phase))
			}
			return fmt.Errorf("%d PersistentVolumeClaims left in the cluster\n%s", count, strings.TrimSuffix(builder.String(), "\n"))
		}
		log.InfoLn("All PersistentVolumeClaims are deleted from the cluster")
		return nil
	})
}

func checkMachinesAPI(kubeCl *client.KubernetesClient, gv schema.GroupVersion) error {
	resourcesList, err := kubeCl.Discovery().ServerResourcesForGroupVersion(gv.String())
	if err != nil {
		return err
	}

	var desiredResources int
	for _, resource := range resourcesList.APIResources {
		if resource.Kind == "Machine" || resource.Kind == "MachineDeployment" {
			desiredResources++
			continue
		}
	}

	if desiredResources < 2 {
		return fmt.Errorf("%d of 2 resources found in the cluster", desiredResources)
	}

	return nil
}

// mcm
const (
	MCMGroup        = "machine.sapcloud.io"
	MCMGroupVersion = "v1alpha1"
)

func DeleteMCMMachineDeployments(ctx context.Context, kubeCl *client.KubernetesClient) error {
	machineDeploymentsSchema := schema.GroupVersionResource{Group: MCMGroup, Version: MCMGroupVersion, Resource: "machinedeployments"}
	machinesSchema := schema.GroupVersionResource{Group: MCMGroup, Version: MCMGroupVersion, Resource: "machines"}

	return retry.NewLoop("Delete MCM MachineDeployments", 45, 5*time.Second).RunContext(ctx, func() error {
		allMachines, err := kubeCl.Dynamic().Resource(machinesSchema).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("get machines: %v", err)
		}

		for _, machine := range allMachines.Items {
			labels := machine.GetLabels()
			// it needs for force delete machine (without drain)
			labels["force-deletion"] = "True"
			machine.SetLabels(labels)

			content, err := machine.MarshalJSON()
			if err != nil {
				return err
			}

			_, err = kubeCl.Dynamic().Resource(machinesSchema).Namespace(machine.GetNamespace()).Patch(ctx, machine.GetName(), types.MergePatchType, content, metav1.PatchOptions{})
			if err != nil {
				return fmt.Errorf("patch machine %s: %v", machine.GetName(), err)
			}
		}

		allMachineDeployments, err := kubeCl.Dynamic().Resource(machineDeploymentsSchema).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("get machinedeployments: %v", err)
		}

		for _, machineDeployment := range allMachineDeployments.Items {
			namespace := machineDeployment.GetNamespace()
			name := machineDeployment.GetName()
			err := kubeCl.Dynamic().Resource(machineDeploymentsSchema).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("delete machinedeployments %s: %v", name, err)
			}
			log.InfoF("%s/%s\n", namespace, name)
		}
		return nil
	})
}

func WaitForMCMMachinesDeletion(ctx context.Context, kubeCl *client.KubernetesClient) error {
	resourceSchema := schema.GroupVersionResource{Group: MCMGroup, Version: MCMGroupVersion, Resource: "machines"}
	return retry.NewLoop("Wait for MCM Machines deletion", 45, 15*time.Second).WithShowError(false).RunContext(ctx, func() error {
		resources, err := kubeCl.Dynamic().Resource(resourceSchema).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		count := len(resources.Items)
		if count != 0 {
			builder := strings.Builder{}
			for _, item := range resources.Items {
				builder.WriteString(fmt.Sprintf("\t\t%s/%s\n", item.GetNamespace(), item.GetName()))
			}
			return fmt.Errorf("%d Machines left in the cluster\n%s", count, strings.TrimSuffix(builder.String(), "\n"))
		}
		log.InfoLn("All Machines are deleted from the cluster")
		return nil
	})
}

func checkMCMMachinesAPI(kubeCl *client.KubernetesClient) error {
	gv := schema.GroupVersion{
		Group:   MCMGroup,
		Version: MCMGroupVersion,
	}

	return checkMachinesAPI(kubeCl, gv)
}

func DeleteMachinesIfResourcesExist(ctx context.Context, kubeCl *client.KubernetesClient) error {
	err := retry.NewLoop("Get Kubernetes cluster resources for MCM group/version", 5, 5*time.Second).WithShowError(false).
		RunContext(ctx, func() error {
			return checkMCMMachinesAPI(kubeCl)
		})
	if err != nil {
		log.WarnF("Can't get resources in group=machine.sapcloud.io, version=v1alpha1: %v\n", err)
		if input.NewConfirmation().
			WithMessage("Machines weren't deleted from the cluster. Do you want to continue?").
			WithYesByDefault().
			Ask() {
			return nil
		}
		return fmt.Errorf("Machines deletion aborted.\n")
	}

	err = DeleteMCMMachineDeployments(ctx, kubeCl)
	if err != nil {
		return err
	}

	if err := WaitForMCMMachinesDeletion(ctx, kubeCl); err != nil {
		return err
	}

	// try to remove CAPI machines it needs for static clusters and cluster with cluster api support
	err = retry.NewLoop("Get Kubernetes cluster resources for CAPI group/version", 5, 5*time.Second).WithShowError(false).
		RunContext(ctx, func() error {
			return checkCAPIMachinesAPI(kubeCl)
		})
	if err != nil {
		log.WarnF("Can't get resources in group=cluster.x-k8s.io, version=v1beta1: %v\n", err)
		if input.NewConfirmation().
			WithMessage("Machines weren't deleted from the cluster. Do you want to continue?").
			WithYesByDefault().
			Ask() {
			return nil
		}
		return fmt.Errorf("Machines deletion aborted.\n")
	}

	err = DeleteCAPIMachineDeployments(ctx, kubeCl)
	if err != nil {
		return err
	}

	return WaitForCAPIMachinesDeletion(ctx, kubeCl)
}

// CAPI
const (
	CAPIGroup        = "cluster.x-k8s.io"
	CAPIGroupVersion = "v1beta1"
)

var capiMachinesSchema = schema.GroupVersionResource{Group: CAPIGroup, Version: CAPIGroupVersion, Resource: "machines"}

func checkCAPIMachinesAPI(kubeCl *client.KubernetesClient) error {
	gv := schema.GroupVersion{
		Group:   CAPIGroup,
		Version: CAPIGroupVersion,
	}

	return checkMachinesAPI(kubeCl, gv)
}

func DeleteCAPIMachineDeployments(ctx context.Context, kubeCl *client.KubernetesClient) error {
	machineDeploymentsSchema := schema.GroupVersionResource{Group: CAPIGroup, Version: CAPIGroupVersion, Resource: "machinedeployments"}

	return retry.NewLoop("Delete CAPI MachineDeployments", 45, 5*time.Second).RunContext(ctx, func() error {
		allMachines, err := kubeCl.Dynamic().Resource(capiMachinesSchema).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("get machines: %v", err)
		}

		for _, machine := range allMachines.Items {
			log.DebugF("Patch nodeDrainTimeout for machine %s\n", machine.GetName())
			m := machine
			// we delete cluster anyway and we can force delete machine (without drain)
			unstructured.SetNestedField(m.Object, "10s", "spec", "nodeDrainTimeout")

			_, err = kubeCl.Dynamic().Resource(capiMachinesSchema).Namespace(machine.GetNamespace()).Update(ctx, &m, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("patch machine %s: %v", machine.GetName(), err)
			}

			log.DebugF("Machine %s patched\n", machine.GetName())
		}

		allMachineDeployments, err := kubeCl.Dynamic().Resource(machineDeploymentsSchema).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("get machinedeployments: %v", err)
		}

		for _, machineDeployment := range allMachineDeployments.Items {
			namespace := machineDeployment.GetNamespace()
			name := machineDeployment.GetName()
			if name == "master" {
				log.InfoLn("Machine deployment 'master' was skipped. It will be deleted later.")
				continue
			}
			err := kubeCl.Dynamic().Resource(machineDeploymentsSchema).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("delete machinedeployments %s: %v", name, err)
			}
			log.InfoF("%s/%s\n", namespace, name)
		}
		return nil
	})
}

func WaitForCAPIMachinesDeletion(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Wait for CAPI Machines deletion", 45, 15*time.Second).WithShowError(false).RunContext(ctx, func() error {
		resources, err := kubeCl.Dynamic().Resource(capiMachinesSchema).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		machines := make([]unstructured.Unstructured, 0, len(resources.Items))
		for _, m := range resources.Items {
			labels := m.GetLabels()
			if labels != nil {
				ng, ok := labels["node-group"]
				if ok && ng == "master" {
					log.DebugF("Machine %s was skipped from delete check because it is in master ng. Continue.\n", m.GetName())
					continue
				}
			}

			machines = append(machines, m)
		}

		count := len(machines)
		if count != 0 {
			builder := strings.Builder{}
			for _, item := range machines {
				builder.WriteString(fmt.Sprintf("\t\t%s/%s\n", item.GetNamespace(), item.GetName()))
			}
			return fmt.Errorf("%d CAPI Machines left in the cluster\n%s", count, strings.TrimSuffix(builder.String(), "\n"))
		}
		log.InfoLn("All CAPI Machines are deleted from the cluster")
		return nil
	})
}
