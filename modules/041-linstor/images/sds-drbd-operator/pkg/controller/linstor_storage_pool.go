/*
Copyright 2023 Flant JSC

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

package controller

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sds-drbd-operator/api/v1alpha1"
	"time"

	lclient "github.com/LINBIT/golinstor/client"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	LinstorStoragePoolControllerName = "linstor-storage-pool-controller"
	TypeLVMThin                      = "LVMThin"
	TypeLVM                          = "LVM"
)

func NewLinstorStoragePool(
	ctx context.Context,
	mgr manager.Manager,
	lc *lclient.Client,
) (controller.Controller, error) {
	cl := mgr.GetClient()
	log := mgr.GetLogger()

	c, err := controller.New(LinstorStoragePoolControllerName, mgr, controller.Options{
		Reconciler: reconcile.Func(func(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
			return reconcile.Result{}, nil
		}),
	})

	if err != nil {
		return nil, err
	}

	err = c.Watch(
		source.Kind(mgr.GetCache(), &v1alpha1.LinstorStoragePool{}),
		handler.Funcs{
			CreateFunc: func(ctx context.Context, e event.CreateEvent, q workqueue.RateLimitingInterface) {
				// ----------------------- get LinstorStoragePool ----------------------------------
				lsp, err := GetLinstorStoragePool(ctx, cl, e.Object.GetNamespace(), e.Object.GetName())
				if err != nil {
					log.Error(err, "error get LinstorStoragePool ")
					return
				}
				// ----------------------- get LinstorStorageClass ----------------------------------

				if lsp.Status.Phase == "Completed" {
					log.Info("linstor pool " + lsp.Name + " " + lsp.Status.Phase)
					return
				}

				ok, msg := ValidateVolumeGroup(ctx, cl, lsp)
				if !ok {
					lsp.Status.Phase = "Failed"
					lsp.Status.Reason = fmt.Sprintf("%v", msg)
					err = UpdateLinstorStoragePool(ctx, cl, lsp)
					if err != nil {
						log.Error(err, "error UpdateLinstorStoragePool")
					}
					return
				}

				var name, vg string
				var lvmType lclient.ProviderKind

				for _, gn := range lsp.Spec.LvmVolumeGroups {
					if lsp.Spec.Type == TypeLVMThin && len(gn.ThinPoolName) == 0 {
						log.Error(errors.New("linstor storage pool"), "type TypeLVMThin but ThinPoolName not set")
						lsp.Status.Phase = "Failed"
						lsp.Status.Reason = "type TypeLVMThin but ThinPoolName not set"
						err = UpdateLinstorStoragePool(ctx, cl, lsp)
						if err != nil {
							log.Error(err, "error UpdateLinstorStoragePool")
						}
						return
					}

					switch lsp.Spec.Type {
					case TypeLVM:
						name = gn.Name

					case TypeLVMThin:
						name = gn.ThinPoolName
					}

					group, err := GetLvmVolumeGroup(ctx, cl, e.Object.GetNamespace(), name)
					if err != nil {
						log.Error(err, "error GetLvmVolumeGroup")
						return
					}

					if len(group.Status.Nodes) != 1 {
						lsp.Status.Phase = "Failed"
						lsp.Status.Reason = "group.Status.Nodes > 1"
						err = UpdateLinstorStoragePool(ctx, cl, lsp)
						if err != nil {
							log.Error(err, "error UpdateLinstorStoragePool")
						}
						return
					}

					switch lsp.Spec.Type {
					case TypeLVM:
						name = gn.Name
						lvmType = lclient.LVM
						vg = group.Spec.ActuaLvgOnTheNode

					case TypeLVMThin:
						name = gn.ThinPoolName
						lvmType = lclient.LVM_THIN
						vg = group.Spec.ActuaLvgOnTheNode + "/thin" + group.Spec.ThinPool.Name
					}

					log.Info("========== Create Storage Pool Data =============")
					log.Info("linstor storage pool name = " + lsp.Name)
					log.Info("node = " + group.Status.Nodes[0].Name)
					log.Info("VG = " + group.Spec.ActuaLvgOnTheNode)
					log.Info("========== ======================== =============")

					// ------------------------ CreateStoragePool ------------------------
					storagePool := lclient.StoragePool{
						StoragePoolName: lsp.Name,
						NodeName:        group.Status.Nodes[0].Name,
						ProviderKind:    lvmType,
						Props: map[string]string{
							"StorDriver/LvmVg": vg,
						},
					}

					start := time.Now()
					lspGet, err := lc.Nodes.GetStoragePool(ctx, group.Status.Nodes[0].Name, lsp.Name)
					if err == lclient.NotFoundError {

						fmt.Println("lspGet.NodeName =======>  ", lspGet.NodeName)
						if lspGet.NodeName == group.Status.Nodes[0].Name {
							lsp.Status.Phase = "Failed"
							lsp.Status.Reason = lspGet.NodeName + " node has already been used"
							err = UpdateLinstorStoragePool(ctx, cl, lsp)
							if err != nil {
								log.Error(err, lspGet.NodeName+" node has already been used")
							}
						}

						log.Info("creating pool " + lsp.Name)
						err = lc.Nodes.CreateStoragePool(ctx, group.Status.Nodes[0].Name, storagePool)
						if err != nil {
							log.Error(err, "CreateStoragePool")

							err := lc.Nodes.DeleteStoragePool(ctx, group.Status.Nodes[0].Name, lsp.Name)
							if err != nil {
								log.Error(err, "DeleteStoragePool")
							}

							lsp.Status.Phase = "Failed"
							log.Info("lsp status phase = " + lsp.Status.Phase)
							log.Info("deleting storage pool " + lsp.Name)
							err = UpdateLinstorStoragePool(ctx, cl, lsp)
							if err != nil {
								log.Error(err, "error UpdateLinstorStoragePool")
							}
							log.Info("deleted storage pool " + lsp.Name)
							return
						}
					}

					duration := time.Since(start)
					log.Info("time spent on pool creation : " + duration.String())
					log.Info("pool created " + lsp.Name)

					lsp.Status.Phase = "Completed"
					lsp.Status.Reason = "pool creation completed"
					err = UpdateLinstorStoragePool(ctx, cl, lsp)
					if err != nil {
						log.Error(err, "")
					}
					log.Info("lsp status updated ")

					// ------------------------ CreateStoragePool ------------------------
				}
			},
			UpdateFunc: func(ctx context.Context, u event.UpdateEvent, limitingInterface workqueue.RateLimitingInterface) {
				newLSP, ok := u.ObjectNew.(*v1alpha1.LinstorStoragePool)
				if !ok {
					log.Error(err, "error get  ObjectNew LinstorStoragePool")
				}

				oldLSP, ok := u.ObjectOld.(*v1alpha1.LinstorStoragePool)
				if !ok {
					log.Error(err, "error get  ObjectOld LinstorStoragePool")
				}

				if reflect.DeepEqual(oldLSP.Spec, newLSP.Spec) {
					return
				}

				if oldLSP.Spec.Type != newLSP.Spec.Type {
					newLSP.Status.Phase = "Failed"
					newLSP.Status.Reason = "Can't change the LVM type"
					err = UpdateLinstorStoragePool(ctx, cl, newLSP)
					fmt.Println("----------- oldLSP.Spec.Type != newLSP.Spec.Type ----------- ")
					if err != nil {
						fmt.Println("errror UpdateLinstorStoragePool oldLSP.Spec.Type != newLSP.Spec.Type ")
						fmt.Println(err)
						log.Error(err, "UpdateLinstorStoragePool")
					}
					return
				}

				var vg string
				var lvmType lclient.ProviderKind

				log.Info("-------------------- LVM Group ---------------------------")
				for _, og := range oldLSP.Spec.LvmVolumeGroups {
					switch oldLSP.Spec.Type {
					case TypeLVM:
						log.Info("LvmVolumeGroupsNames: new groups LVM " + og.Name)
					case TypeLVMThin:
						log.Info("LvmVolumeGroupsNames: new groups Thin LVM" + og.ThinPoolName)
					}

				}

				for _, ng := range newLSP.Spec.LvmVolumeGroups {
					switch newLSP.Spec.Type {
					case TypeLVM:
						log.Info("LvmVolumeGroupsNames: new groups LVM " + ng.Name)
					case TypeLVMThin:
						log.Info("LvmVolumeGroupsNames: new groups Thin LVM" + ng.Name + " with thin pool: " + ng.ThinPoolName)
					}
				}
				log.Info("-------------------- LVM Group ---------------------------")

				if len(newLSP.Spec.LvmVolumeGroups) > len(oldLSP.Spec.LvmVolumeGroups) {

					for _, ng := range newLSP.Spec.LvmVolumeGroups {
						group, err := GetLvmVolumeGroup(ctx, cl, newLSP.GetNamespace(), ng.Name)
						if err != nil {
							log.Error(err, "error GetLvmVolumeGroup")
							return
						}

						if newLSP.Spec.Type != oldLSP.Spec.Type {
							switch newLSP.Spec.Type {
							case TypeLVM:
								lvmType = lclient.LVM
								vg = group.Spec.ActuaLvgOnTheNode
							case TypeLVMThin:
								lvmType = lclient.LVM_THIN
								vg = group.Spec.ActuaLvgOnTheNode + "/thin" + group.Spec.ThinPool.Name
							}
						}

						if len(group.Status.Nodes) != 1 {
							newLSP.Status.Phase = "Failed"
							newLSP.Status.Reason = "group.Status.Nodes > 1"
							err = UpdateLinstorStoragePool(ctx, cl, newLSP)
							if err != nil {
								log.Error(err, "error UpdateLinstorStoragePool")
							}
							return
						}

						log.Info("========== +++++++++++++ =============")
						log.Info("linstor storage pool name = ", newLSP.Name)
						log.Info("node len=", len(group.Status.Nodes))
						log.Info("node =", group.Status.Nodes[0].Name)
						log.Info("VG = ", group.Spec.ActuaLvgOnTheNode)
						log.Info("========== +++++++++++++ =============")

						// ------------------------ CreateStoragePool ------------------------
						storagePool := lclient.StoragePool{
							StoragePoolName: newLSP.Name,
							NodeName:        group.Status.Nodes[0].Name,
							ProviderKind:    lvmType,
							Props: map[string]string{
								"StorDriver/LvmVg": vg,
							},
						}

						err = lc.Nodes.CreateStoragePool(ctx, group.Status.Nodes[0].Name, storagePool)
						if err != nil {
							log.Error(err, "CreateStoragePool")
							newLSP.Status.Phase = "Failed"
							newLSP.Status.Reason = "Failed CreateStoragePool"
							err = UpdateLinstorStoragePool(ctx, cl, newLSP)
							if err != nil {
								log.Error(err, "error UpdateLinstorStoragePool")
							}
							return
						}

						// ------------------------ CreateStoragePool ------------------------
					}

					newLSP.Status.Phase = "Completed"
					err = UpdateLinstorStoragePool(ctx, cl, newLSP)
					if err != nil {
						log.Error(err, "")
					}

				} else {
					newLSP.Status.Phase = "Failed"
					err = UpdateLinstorStoragePool(ctx, cl, newLSP)
					if err != nil {
						log.Error(err, "error UpdateLinstorStoragePool")
					}
					return
				}
			},
			DeleteFunc: nil,
		})
	if err != nil {
		return nil, err
	}
	return c, err
}

func UpdateLinstorStoragePool(ctx context.Context, cl client.Client, lsc *v1alpha1.LinstorStoragePool) error {
	err := cl.Update(ctx, lsc)
	if err != nil {
		return err
	}
	return nil
}

func GetLinstorStoragePool(ctx context.Context, cl client.Client, namespace, name string) (*v1alpha1.LinstorStoragePool, error) {
	obj := &v1alpha1.LinstorStoragePool{}
	err := cl.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, obj)
	if err != nil {
		return nil, err
	}
	return obj, err
}

func GetLvmVolumeGroup(ctx context.Context, cl client.Client, namespace, name string) (*v1alpha1.LvmVolumeGroup, error) {
	obj := &v1alpha1.LvmVolumeGroup{}
	err := cl.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, obj)
	if err != nil {
		return nil, err
	}
	return obj, err
}

func ValidateVolumeGroup(ctx context.Context, cl client.Client, lsp *v1alpha1.LinstorStoragePool) (bool, map[string]string) {
	var lvmVolumeGroupName string
	var nodeName string
	nodesWithlvmVolumeGroups := make(map[string]string)
	invalidLvmVolumeGroups := make(map[string]string)
	lvmVolumeGroupsNames := make(map[string]bool)

	for _, g := range lsp.Spec.LvmVolumeGroups {
		lvmVolumeGroupName = g.Name

		if lvmVolumeGroupsNames[lvmVolumeGroupName] {
			//UpdateMapValue(invalidLvmVolumeGroups, lvmVolumeGroupName, fmt.Sprintf("LvmVolumeGroup name is not unique, %v", lvmVolumeGroupsNames[lvmVolumeGroupName]))
			invalidLvmVolumeGroups[lvmVolumeGroupName] = "LvmVolumeGroup name is not unique"
			continue
		}
		lvmVolumeGroupsNames[lvmVolumeGroupName] = true

		group, err := GetLvmVolumeGroup(ctx, cl, lsp.Namespace, lvmVolumeGroupName)
		if err != nil {
			UpdateMapValue(invalidLvmVolumeGroups, lvmVolumeGroupName, fmt.Sprintf("Error getting LVMVolumeGroup: %s", err.Error()))
			continue
		}

		if len(group.Status.Nodes) != 1 {
			UpdateMapValue(invalidLvmVolumeGroups, lvmVolumeGroupName, "expected LvmVolumeGroup for LINSTOR Storage Pool to have only one node")
		} else {

			nodeName = group.Status.Nodes[0].Name

			if value, ok := nodesWithlvmVolumeGroups[nodeName]; ok {
				UpdateMapValue(invalidLvmVolumeGroups, lvmVolumeGroupName, fmt.Sprintf("This LvmVolumeGroup have same node %s as LvmVolumeGroup with name: %s. This is forbidden", nodeName, value))
				continue
			}

			nodesWithlvmVolumeGroups[nodeName] = lvmVolumeGroupName
		}
	}

	if len(invalidLvmVolumeGroups) > 0 {
		return false, invalidLvmVolumeGroups
	}

	return true, nil
}

func UpdateMapValue(m map[string]string, key string, additionalValue string) {
	if oldValue, ok := m[key]; ok {
		m[key] = fmt.Sprintf("%s. Also: %s", oldValue, additionalValue)
	} else {
		m[key] = additionalValue
	}
}
