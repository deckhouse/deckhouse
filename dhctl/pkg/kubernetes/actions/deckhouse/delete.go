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

func DeleteDeckhouseDeployment(kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Delete Deckhouse", 45, 5*time.Second).WithShowError(false).Run(func() error {
		foregroundPolicy := metav1.DeletePropagationForeground
		err := kubeCl.AppsV1().Deployments(deckhouseDeploymentNamespace).Delete(context.TODO(), deckhouseDeploymentName, metav1.DeleteOptions{PropagationPolicy: &foregroundPolicy})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		return nil
	})
}

func DeleteStorageClasses(kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Delete StorageClasses", 45, 5*time.Second).WithShowError(false).Run(func() error {
		return kubeCl.StorageV1().StorageClasses().DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{})
	})
}

func DeletePods(kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Delete Pods", 45, 5*time.Second).WithShowError(false).Run(func() error {
		pods, err := kubeCl.CoreV1().Pods(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
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

			err := kubeCl.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
			if err != nil {
				log.ErrorLn(err.Error())
			} else {
				log.InfoF("%s/%s\n", pod.Namespace, pod.Name)
			}
		}

		return nil
	})
}

func DeleteServices(kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Delete Services", 45, 5*time.Second).WithShowError(false).Run(func() error {
		allServices, err := kubeCl.CoreV1().Services(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, service := range allServices.Items {
			if service.Spec.Type != v1.ServiceTypeLoadBalancer {
				continue
			}

			err := kubeCl.CoreV1().Services(service.Namespace).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
			log.InfoF("%s/%s\n", service.Namespace, service.Name)
		}
		return nil
	})
}

func DeletePVC(kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Delete PersistentVolumeClaims", 45, 5*time.Second).WithShowError(false).Run(func() error {
		volumeClaims, err := kubeCl.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, claim := range volumeClaims.Items {
			err := kubeCl.CoreV1().PersistentVolumeClaims(claim.Namespace).Delete(context.TODO(), claim.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
			log.InfoF("%s/%s\n", claim.Namespace, claim.Name)
		}
		return nil
	})
}

func WaitForDeckhouseDeploymentDeletion(kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Wait for Deckhouse Deployment deletion", 30, 5*time.Second).WithShowError(false).Run(func() error {
		_, err := kubeCl.AppsV1().Deployments(deckhouseDeploymentNamespace).Get(context.TODO(), deckhouseDeploymentName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			log.InfoLn("Deckhouse Deployment and its dependents are removed")
			return nil
		}

		errStr := "Deckhouse Deployment and its dependents are not removed from the cluster yet"
		if err != nil {
			errStr = fmt.Sprintf("Error during waiting, err: %v", err)
		}
		//goland:noinspection GoErrorStringFormat
		return fmt.Errorf(errStr)
	})
}

func WaitForServicesDeletion(kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Wait for Services deletion", 45, 15*time.Second).WithShowError(false).Run(func() error {
		resources, err := kubeCl.CoreV1().Services(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
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

func WaitForPVDeletion(kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Wait for PersistentVolumes deletion", 45, 15*time.Second).WithShowError(false).Run(func() error {
		resources, err := kubeCl.CoreV1().PersistentVolumes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return err
		}

		count := len(resources.Items)
		if count != 0 {
			var (
				pvsWithnonDeleteReclaimPolicy strings.Builder
				remainingPVs                  strings.Builder
			)

			for _, item := range resources.Items {
				if item.Spec.PersistentVolumeReclaimPolicy != v1.PersistentVolumeReclaimDelete {
					pvsWithnonDeleteReclaimPolicy.WriteString(fmt.Sprintf("\t\t%s | %s\n", item.Name, item.Status.Phase))
				}

				remainingPVs.WriteString(fmt.Sprintf("\t\t%s | %s\n", item.Name, item.Status.Phase))
			}

			if pvsWithnonDeleteReclaimPolicy.Len() != 0 {
				return fmt.Errorf("%d PersistentVolumes with reclaimPolicy other than Delete in the cluster. Set their reclaim policy to Delete or remove them manually\n%s",
					count, strings.TrimSuffix(remainingPVs.String(), "\n"))
			}

			return fmt.Errorf("%d PersistentVolumes left in the cluster\n%s", count, strings.TrimSuffix(remainingPVs.String(), "\n"))
		}
		log.InfoLn("All PersistentVolumes are deleted from the cluster")
		return nil
	})
}

func WaitForPVCDeletion(kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Wait for PersistentVolumeClaims deletion", 45, 15*time.Second).WithShowError(false).Run(func() error {
		resources, err := kubeCl.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
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

func DeleteMCMMachineDeployments(kubeCl *client.KubernetesClient) error {
	machineDeploymentsSchema := schema.GroupVersionResource{Group: MCMGroup, Version: MCMGroupVersion, Resource: "machinedeployments"}
	machinesSchema := schema.GroupVersionResource{Group: MCMGroup, Version: MCMGroupVersion, Resource: "machines"}

	return retry.NewLoop("Delete MCM MachineDeployments", 45, 5*time.Second).Run(func() error {
		allMachines, err := kubeCl.Dynamic().Resource(machinesSchema).Namespace(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
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

			_, err = kubeCl.Dynamic().Resource(machinesSchema).Namespace(machine.GetNamespace()).Patch(context.TODO(), machine.GetName(), types.MergePatchType, content, metav1.PatchOptions{})
			if err != nil {
				return fmt.Errorf("patch machine %s: %v", machine.GetName(), err)
			}
		}

		allMachineDeployments, err := kubeCl.Dynamic().Resource(machineDeploymentsSchema).Namespace(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("get machinedeployments: %v", err)
		}

		for _, machineDeployment := range allMachineDeployments.Items {
			namespace := machineDeployment.GetNamespace()
			name := machineDeployment.GetName()
			err := kubeCl.Dynamic().Resource(machineDeploymentsSchema).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("delete machinedeployments %s: %v", name, err)
			}
			log.InfoF("%s/%s\n", namespace, name)
		}
		return nil
	})
}

func WaitForMCMMachinesDeletion(kubeCl *client.KubernetesClient) error {
	resourceSchema := schema.GroupVersionResource{Group: MCMGroup, Version: MCMGroupVersion, Resource: "machines"}
	return retry.NewLoop("Wait for MCM Machines deletion", 45, 15*time.Second).WithShowError(false).Run(func() error {
		resources, err := kubeCl.Dynamic().Resource(resourceSchema).List(context.TODO(), metav1.ListOptions{})
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

func DeleteMachinesIfResourcesExist(kubeCl *client.KubernetesClient) error {
	err := retry.NewLoop("Get Kubernetes cluster resources for MCM group/version", 5, 5*time.Second).WithShowError(false).
		Run(func() error {
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

	err = DeleteMCMMachineDeployments(kubeCl)
	if err != nil {
		return err
	}

	if err := WaitForMCMMachinesDeletion(kubeCl); err != nil {
		return err
	}

	// try to remove CAPI machines it needs for static clusters and cluster with cluster api support
	err = retry.NewLoop("Get Kubernetes cluster resources for CAPI group/version", 5, 5*time.Second).WithShowError(false).
		Run(func() error {
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

	err = DeleteCAPIMachineDeployments(kubeCl)
	if err != nil {
		return err
	}

	return WaitForCAPIMachinesDeletion(kubeCl)
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

func DeleteCAPIMachineDeployments(kubeCl *client.KubernetesClient) error {
	machineDeploymentsSchema := schema.GroupVersionResource{Group: CAPIGroup, Version: CAPIGroupVersion, Resource: "machinedeployments"}

	return retry.NewLoop("Delete CAPI MachineDeployments", 45, 5*time.Second).Run(func() error {
		allMachines, err := kubeCl.Dynamic().Resource(capiMachinesSchema).Namespace(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("get machines: %v", err)
		}

		for _, machine := range allMachines.Items {
			m := machine
			// we delete cluster anyway and we can force delete machine (without drain)
			unstructured.SetNestedField(m.Object, "10s", "spec", "nodeDrainTimeout")

			_, err = kubeCl.Dynamic().Resource(capiMachinesSchema).Namespace(machine.GetNamespace()).Update(context.TODO(), &m, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("patch machine %s: %v", machine.GetName(), err)
			}
		}

		allMachineDeployments, err := kubeCl.Dynamic().Resource(machineDeploymentsSchema).Namespace(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("get machinedeployments: %v", err)
		}

		for _, machineDeployment := range allMachineDeployments.Items {
			namespace := machineDeployment.GetNamespace()
			name := machineDeployment.GetName()
			err := kubeCl.Dynamic().Resource(machineDeploymentsSchema).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("delete machinedeployments %s: %v", name, err)
			}
			log.InfoF("%s/%s\n", namespace, name)
		}
		return nil
	})
}

func WaitForCAPIMachinesDeletion(kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Wait for CAPI Machines deletion", 45, 15*time.Second).WithShowError(false).Run(func() error {
		resources, err := kubeCl.Dynamic().Resource(capiMachinesSchema).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return err
		}

		count := len(resources.Items)
		if count != 0 {
			builder := strings.Builder{}
			for _, item := range resources.Items {
				builder.WriteString(fmt.Sprintf("\t\t%s/%s\n", item.GetNamespace(), item.GetName()))
			}
			return fmt.Errorf("%d CAPI Machines left in the cluster\n%s", count, strings.TrimSuffix(builder.String(), "\n"))
		}
		log.InfoLn("All CAPI Machines are deleted from the cluster")
		return nil
	})
}
