package controller

import (
	"context"
	"fmt"
	lclient "github.com/LINBIT/golinstor/client"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"st2/api/v1alpha1"
	"time"
)

const (
	LinstorStoragePoolControllerName = "linstor-storage-pool-controller"
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
				log.Info("get LinstorStoragPool " + e.Object.GetName())
				lsp, err := getLinstorStoragePool(ctx, cl, e.Object.GetNamespace(), e.Object.GetName())
				if err != nil {
					log.Error(err, "error get LinstorStoragePool ")
					return
				}
				// ----------------------- get LinstorStorageClass ----------------------------------

				// ----------------------- update LinstorStoragePool --------------------------------
				if Double(lsp.Spec.LvmVolumeGroupsNames) {
					lsp.Status.Phase = "Failed"
					lsp.Status.Reason = "lvmVolumeGroupsNames is contains doubles"
					err = updateLinstorStoragePool(ctx, cl, lsp)
					if err != nil {
						log.Error(err, "error updateLinstorStoragePool")
					}
					return
				}

				for _, gn := range lsp.Spec.LvmVolumeGroupsNames {
					group, err := getLvmVolumeGroup(ctx, cl, e.Object.GetNamespace(), gn)
					if err != nil {
						log.Error(err, "error getLvmVolumeGroup")
						return
					}

					fmt.Println("-----------------------------------------------------------")
					fmt.Println("group.Spec.ActuaLvgOnTheNode", group.Spec.ActuaLvgOnTheNode)
					fmt.Println("group.Status.Nodes", group.Status.Nodes)
					fmt.Println("-----------------------------------------------------------")

					if len(group.Status.Nodes) != 1 {
						lsp.Status.Phase = "Failed"
						lsp.Status.Reason = "group.Status.Nodes > 1"
						err = updateLinstorStoragePool(ctx, cl, lsp)
						if err != nil {
							log.Error(err, "error updateLinstorStoragePool")
						}
						return
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
						ProviderKind:    lclient.LVM,
						Props: map[string]string{
							"StorDriver/LvmVg": group.Spec.ActuaLvgOnTheNode,
						},
					}

					start := time.Now()

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

					duration := time.Since(start)
					log.Info("create time : " + duration.String())
					log.Info("created storage pool " + lsp.Name)

					lsp.Status.Phase = "Completed"
					log.Info("lsp status phase = " + lsp.Status.Phase)
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

				if len(newLSP.Spec.LvmVolumeGroupsNames) > len(oldLSP.Spec.LvmVolumeGroupsNames) {

					if Double(newLSP.Spec.LvmVolumeGroupsNames) {
						newLSP.Status.Phase = "Failed"
						newLSP.Status.Reason = "lvmVolumeGroupsNames is contains doubles"
						err = updateLinstorStoragePool(ctx, cl, newLSP)
						if err != nil {
							log.Error(err, "error updateLinstorStoragePool")
						}
						return
					}

					for _, gn := range newLSP.Spec.LvmVolumeGroupsNames {
						fmt.Println("gn ", gn)
						group, err := getLvmVolumeGroup(ctx, cl, newLSP.GetNamespace(), gn)
						if err != nil {
							log.Error(err, "error getLvmVolumeGroup")
							return
						}

						fmt.Println("-----------------------------------------------------------")
						fmt.Println("group.Spec.ActuaLvgOnTheNode", group.Spec.ActuaLvgOnTheNode)
						fmt.Println("group.Status.Nodes", group.Status.Nodes)
						fmt.Println("-----------------------------------------------------------")

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
							ProviderKind:    lclient.LVM,
							Props: map[string]string{
								"StorDriver/LvmVg": group.Spec.ActuaLvgOnTheNode,
							},
						}

						err = lc.Nodes.CreateStoragePool(ctx, group.Status.Nodes[0].Name, storagePool)
						if err != nil {
							log.Error(err, "CreateStoragePool")
							newLSP.Status.Phase = "Failed"
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
