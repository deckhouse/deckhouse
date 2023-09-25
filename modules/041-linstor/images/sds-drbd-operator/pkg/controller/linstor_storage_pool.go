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
	lclient "github.com/LINBIT/golinstor/client"
	"k8s.io/client-go/util/workqueue"
	"reflect"
	"sds-drbd-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
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
				lsp, err := getLinstorStoragePool(ctx, cl, e.Object.GetNamespace(), e.Object.GetName())
				if err != nil {
					log.Error(err, "error get LinstorStoragePool ")
					return
				}
				// ----------------------- get LinstorStorageClass ----------------------------------

				if lsp.Status.Phase == "Completed" {
					log.Info("linstor pool " + lsp.Name + " " + lsp.Status.Phase)
					return
				}

				doubleNodes, err := validateVolumeGroup(ctx, cl, lsp)
				if err != nil || doubleNodes == 1 {
					lsp.Status.Phase = "Failed"
					lsp.Status.Reason = "lvmVolumeGroupsNames is contains doubles"
					err = updateLinstorStoragePool(ctx, cl, lsp)
					if err != nil {
						log.Error(err, "error updateLinstorStoragePool")
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
						err = updateLinstorStoragePool(ctx, cl, lsp)
						if err != nil {
							log.Error(err, "error updateLinstorStoragePool")
						}
						return
					}

					switch lsp.Spec.Type {
					case TypeLVM:
						name = gn.Name

					case TypeLVMThin:
						name = gn.ThinPoolName
					}

					group, err := getLvmVolumeGroup(ctx, cl, e.Object.GetNamespace(), name)
					if err != nil {
						log.Error(err, "error getLvmVolumeGroup")
						return
					}

					if len(group.Status.Nodes) != 1 {
						lsp.Status.Phase = "Failed"
						lsp.Status.Reason = "group.Status.Nodes > 1"
						err = updateLinstorStoragePool(ctx, cl, lsp)
						if err != nil {
							log.Error(err, "error updateLinstorStoragePool")
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
							err = updateLinstorStoragePool(ctx, cl, lsp)
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
							err = updateLinstorStoragePool(ctx, cl, lsp)
							if err != nil {
								log.Error(err, "error updateLinstorStoragePool")
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
					err = updateLinstorStoragePool(ctx, cl, lsp)
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
					err = updateLinstorStoragePool(ctx, cl, newLSP)
					fmt.Println("----------- oldLSP.Spec.Type != newLSP.Spec.Type ----------- ")
					if err != nil {
						fmt.Println("errror updateLinstorStoragePool oldLSP.Spec.Type != newLSP.Spec.Type ")
						fmt.Println(err)
						log.Error(err, "updateLinstorStoragePool")
					}
					return
				}

				var name, vg string
				var lvmType lclient.ProviderKind

				log.Info("-------------------- LVM Group ---------------------------")
				for _, og := range oldLSP.Spec.LvmVolumeGroups {
					log.Info("LvmVolumeGroupsNames old groups LVM " + og.Name)
					log.Info("LvmVolumeGroupsNames old groups Thin LVM" + og.ThinPoolName)
				}

				for _, ng := range newLSP.Spec.LvmVolumeGroups {
					log.Info("LvmVolumeGroupsNames new groups LVM " + ng.Name)
					log.Info("LvmVolumeGroupsNames new groups Thin LVM" + ng.ThinPoolName)

					switch newLSP.Spec.Type {
					case TypeLVM:
						name = ng.Name
					case TypeLVMThin:
						name = ng.ThinPoolName
					}
				}
				log.Info("-------------------- LVM Group ---------------------------")

				if len(newLSP.Spec.LvmVolumeGroups) > len(oldLSP.Spec.LvmVolumeGroups) {

					for _, gn := range newLSP.Spec.LvmVolumeGroups {
						fmt.Println("gn ", gn)
						group, err := getLvmVolumeGroup(ctx, cl, newLSP.GetNamespace(), name)
						if err != nil {
							log.Error(err, "error getLvmVolumeGroup")
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
							err = updateLinstorStoragePool(ctx, cl, newLSP)
							if err != nil {
								log.Error(err, "error updateLinstorStoragePool")
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
							err = updateLinstorStoragePool(ctx, cl, newLSP)
							if err != nil {
								log.Error(err, "error updateLinstorStoragePool")
							}
							return
						}

						// ------------------------ CreateStoragePool ------------------------
					}

					newLSP.Status.Phase = "Completed"
					err = updateLinstorStoragePool(ctx, cl, newLSP)
					if err != nil {
						log.Error(err, "")
					}

				} else {
					newLSP.Status.Phase = "Failed"
					err = updateLinstorStoragePool(ctx, cl, newLSP)
					if err != nil {
						log.Error(err, "error updateLinstorStoragePool")
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

func updateLinstorStoragePool(ctx context.Context, cl client.Client, lsc *v1alpha1.LinstorStoragePool) error {
	err := cl.Update(ctx, lsc)
	if err != nil {
		return err
	}
	return nil
}

func getLinstorStoragePool(ctx context.Context, cl client.Client, namespace, name string) (*v1alpha1.LinstorStoragePool, error) {
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

func Double(arr []string) bool {
	m := make(map[string]bool)
	for _, v := range arr {
		m[v] = true
	}
	return len(arr) != len(m)
}

func getLvmVolumeGroup(ctx context.Context, cl client.Client, namespace, name string) (*v1alpha1.LvmVolumeGroup, error) {
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

func validateVolumeGroup(ctx context.Context, cl client.Client, lsp *v1alpha1.LinstorStoragePool) (int, error) {
	var name string
	var tempNameNode []string

	for _, g := range lsp.Spec.LvmVolumeGroups {

		switch lsp.Spec.Type {
		case TypeLVM:
			name = g.Name

		case TypeLVMThin:
			name = g.ThinPoolName
		}

		group, err := getLvmVolumeGroup(ctx, cl, lsp.Namespace, name)
		if err != nil {
			return 0, err
		}

		for _, n := range group.Status.Nodes {
			tempNameNode = append(tempNameNode, n.Name)
		}
	}

	if Double(tempNameNode) {
		return 1, nil
	}
	return 0, nil
}
