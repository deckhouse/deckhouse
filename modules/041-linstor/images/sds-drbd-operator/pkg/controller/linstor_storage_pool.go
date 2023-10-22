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
	"bytes"
	"context"
	"fmt"
	"reflect"
	"sds-drbd-operator/api/v1alpha1"
	"sort"
	"time"

	lapi "github.com/LINBIT/golinstor/client"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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
	DRBDOperatorStoragePoolControllerName = "drbd-operator-storage-pool-controller"
	TypeLVMThin                           = "LVMThin"
	TypeLVM                               = "LVM"
	LVMVGTypeLocal                        = "Local"
)

func NewDRBDOperatorStoragePool(
	ctx context.Context,
	mgr manager.Manager,
	lc *lapi.Client,
	interval int,
) (controller.Controller, error) {
	cl := mgr.GetClient()
	log := mgr.GetLogger()

	c, err := controller.New(DRBDOperatorStoragePoolControllerName, mgr, controller.Options{
		Reconciler: reconcile.Func(func(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {

			log.Info("START from reconciler reconcile of DRBDOperator storage pool with name: " + request.Name)

			shouldRequeue, err := ReconcileEvent(ctx, cl, request, log, lc)
			if shouldRequeue {
				log.Error(err, fmt.Sprintf("error in ReconcileEvent. Add to retry after %d seconds.", interval))
				return reconcile.Result{
					RequeueAfter: time.Duration(interval) * time.Second,
				}, err
			}

			log.Info("END from reconciler reconcile of DRBDOperator storage pool with name: " + request.Name)
			return reconcile.Result{Requeue: false}, nil
		}),
	})

	if err != nil {
		return nil, err
	}

	err = c.Watch(
		source.Kind(mgr.GetCache(), &v1alpha1.DRBDOperatorStoragePool{}),
		handler.Funcs{
			CreateFunc: func(ctx context.Context, e event.CreateEvent, q workqueue.RateLimitingInterface) {
				log.Info("START from CREATE reconcile of DRBDOperator storage pool with name: " + e.Object.GetName())

				request := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: e.Object.GetNamespace(), Name: e.Object.GetName()}}
				shouldRequeue, err := ReconcileEvent(ctx, cl, request, log, lc)
				if shouldRequeue {
					log.Error(err, fmt.Sprintf("error in ReconcileEvent. Add to retry after %d seconds.", interval))
					q.AddAfter(request, time.Duration(interval)*time.Second)
				}

				log.Info("END from CREATE reconcile of DRBDOperator storage pool with name: " + request.Name)
			},
			UpdateFunc: func(ctx context.Context, e event.UpdateEvent, q workqueue.RateLimitingInterface) {
				log.Info("START from UPDATE reconcile of DRBDOperator storage pool with name: " + e.ObjectNew.GetName())

				oldDRBDSP := e.ObjectOld.(*v1alpha1.DRBDOperatorStoragePool)
				newDRBDSP := e.ObjectNew.(*v1alpha1.DRBDOperatorStoragePool)
				if reflect.DeepEqual(oldDRBDSP.Spec, newDRBDSP.Spec) {
					log.Info("DRBDOperatorStoragePool spec not changed. Nothing to do")
					return
				}

				request := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: e.ObjectNew.GetNamespace(), Name: e.ObjectNew.GetName()}}
				shouldRequeue, err := ReconcileEvent(ctx, cl, request, log, lc)
				if shouldRequeue {
					log.Error(err, fmt.Sprintf("error in ReconcileEvent. Add to retry after %d seconds.", interval))
					q.AddAfter(request, time.Duration(interval)*time.Second)
				}

				log.Info("END from UPDATE reconcile of DRBDOperator storage pool with name: " + request.Name)
			},
			DeleteFunc: nil,
		})

	return c, err
}

func ReconcileEvent(ctx context.Context, cl client.Client, request reconcile.Request, log logr.Logger, lc *lapi.Client) (bool, error) {
	drbdsp := &v1alpha1.DRBDOperatorStoragePool{}
	err := cl.Get(ctx, request.NamespacedName, drbdsp)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("DRBDOperatorStoragePool with name: " + request.Name + " not found. Object was probably deleted. Remove it from quie as deletion logic not implemented yet.") // #TODO: warn
			return false, nil
		}
		return true, fmt.Errorf("error getting DRBDOperatorStoragePool: %s", err.Error())
	}
	err = ReconcileDRBDOperatorStoragePool(ctx, cl, lc, log, drbdsp)
	if err != nil {
		return true, fmt.Errorf("error ReconcileDRBDOperatorStoragePool: %s", err.Error())
	}
	return false, nil
}

func ReconcileDRBDOperatorStoragePool(ctx context.Context, cl client.Client, lc *lapi.Client, log logr.Logger, drbdsp *v1alpha1.DRBDOperatorStoragePool) error {

	ok, msg, lvmVolumeGroups := GetAndValidateVolumeGroups(ctx, cl, drbdsp.Namespace, drbdsp.Spec.Type, drbdsp.Spec.LvmVolumeGroups)
	if !ok {
		drbdsp.Status.Phase = "Failed"
		drbdsp.Status.Reason = fmt.Sprintf("%v", msg)
		err := UpdateDRBDOperatorStoragePool(ctx, cl, drbdsp)
		if err != nil {
			return fmt.Errorf("error UpdateDRBDOperatorStoragePool: %s", err.Error())
		}
		return nil
	}

	var lvmVgForLinstor string
	var lvmType lapi.ProviderKind

	for _, drbdspLvmVolumeGroup := range drbdsp.Spec.LvmVolumeGroups {
		lvmVolumeGroup, ok := lvmVolumeGroups[drbdspLvmVolumeGroup.Name]
		nodeName := lvmVolumeGroup.Status.Nodes[0].Name

		if !ok {
			drbdsp.Status.Phase = "Failed"
			drbdsp.Status.Reason = fmt.Sprintf("Error getting LvmVolumeGroup %s from lvmVolumeGroups map. See logs of %s for details", drbdspLvmVolumeGroup.Name, DRBDOperatorStoragePoolControllerName)
			return fmt.Errorf("error getting LvmVolumeGroup %s from lvmVolumeGroups map (%v), returned by the GetAndValidateVolumeGroups function", drbdspLvmVolumeGroup.Name, lvmVolumeGroups)
		}

		switch drbdsp.Spec.Type {
		case TypeLVM:
			lvmType = lapi.LVM
			lvmVgForLinstor = lvmVolumeGroup.Spec.ActualVGnameOnTheNode
		case TypeLVMThin:
			lvmType = lapi.LVM_THIN
			lvmVgForLinstor = lvmVolumeGroup.Spec.ActualVGnameOnTheNode + "/" + drbdspLvmVolumeGroup.ThinPoolName
		}

		newStoragePool := lapi.StoragePool{
			StoragePoolName: drbdsp.Name,
			NodeName:        nodeName,
			ProviderKind:    lvmType,
			Props: map[string]string{
				"StorDriver/LvmVg": lvmVgForLinstor, // TODO: change to const
			},
		}

		existedStoragePool, err := lc.Nodes.GetStoragePool(ctx, nodeName, drbdsp.Name)
		if err != nil {
			if err == lapi.NotFoundError {
				log.Info(fmt.Sprintf("Storage Pool %s on node %s on vg %s not found. Creating it", drbdsp.Name, nodeName, lvmVgForLinstor))
				err := lc.Nodes.CreateStoragePool(ctx, nodeName, newStoragePool)
				if err != nil {
					errMessage := fmt.Sprintf("Error creating LINSTOR Storage Pool %s on node %s on vg %s: %s", drbdsp.Name, nodeName, lvmVgForLinstor, err.Error())

					log.Error(nil, errMessage)
					log.Info("Try to delete Storage Pool from LINSTOR if it was mistakenly created")
					err = lc.Nodes.DeleteStoragePool(ctx, nodeName, drbdsp.Name)
					if err != nil {
						log.Error(nil, fmt.Sprintf("Error deleting LINSTOR Storage Pool %s on node %s on vg %s: %s", drbdsp.Name, nodeName, lvmVgForLinstor, err.Error()))
					}

					drbdsp.Status.Phase = "Failed"
					drbdsp.Status.Reason = errMessage
					err := UpdateDRBDOperatorStoragePool(ctx, cl, drbdsp)
					if err != nil {
						return fmt.Errorf("error UpdateDRBDOperatorStoragePool: %s", err.Error())
					}
					return fmt.Errorf("")
				}
				log.Info(fmt.Sprintf("Storage Pool %s created on node %s on vg %s", drbdsp.Name, nodeName, lvmVgForLinstor))
				continue
			} else {
				errMessage := fmt.Sprintf("Error getting LINSTOR Storage Pool %s on node %s on vg %s: %s", drbdsp.Name, nodeName, lvmVgForLinstor, err.Error())
				drbdsp.Status.Phase = "Failed"
				drbdsp.Status.Reason = errMessage
				err := UpdateDRBDOperatorStoragePool(ctx, cl, drbdsp)
				if err != nil {
					log.Error(nil, errMessage)
					return fmt.Errorf("error UpdateDRBDOperatorStoragePool: %s", err.Error())
				}
				return fmt.Errorf(errMessage)
			}
		}

		if existedStoragePool.ProviderKind != newStoragePool.ProviderKind {
			errMessage := fmt.Sprintf("Storage Pool %s on node %s on vg %s already exists but with different type %s. New type is %s. Type change is forbidden", drbdsp.Name, nodeName, lvmVgForLinstor, existedStoragePool.ProviderKind, newStoragePool.ProviderKind)
			drbdsp.Status.Phase = "Failed"
			drbdsp.Status.Reason = errMessage
			err := UpdateDRBDOperatorStoragePool(ctx, cl, drbdsp)
			if err != nil {
				log.Error(nil, errMessage)
				return fmt.Errorf("error UpdateDRBDOperatorStoragePool: %s", err.Error())
			}
			return fmt.Errorf(errMessage)
		}

		if existedStoragePool.Props["StorDriver/LvmVg"] != lvmVgForLinstor {
			errMessage := fmt.Sprintf("Storage Pool %s on node %s already exists with vg \"%s\". New vg is \"%s\". VG change is forbidden", drbdsp.Name, nodeName, existedStoragePool.Props["StorDriver/LvmVg"], lvmVgForLinstor)
			drbdsp.Status.Phase = "Failed"
			drbdsp.Status.Reason = errMessage
			err := UpdateDRBDOperatorStoragePool(ctx, cl, drbdsp)
			if err != nil {
				log.Error(nil, errMessage)
				return fmt.Errorf("error UpdateDRBDOperatorStoragePool: %s", err.Error())
			}
			return fmt.Errorf(errMessage)
		}

		log.Info(fmt.Sprintf("Storage Pool %s on node %s on vg %s already exists. Nothing to do", drbdsp.Name, nodeName, lvmVgForLinstor))
	}

	drbdsp.Status.Phase = "Completed"
	drbdsp.Status.Reason = "pool creation completed"
	err := UpdateDRBDOperatorStoragePool(ctx, cl, drbdsp)
	if err != nil {
		return fmt.Errorf("error UpdateDRBDOperatorStoragePool: %s", err.Error())
	}

	return nil
}

func UpdateDRBDOperatorStoragePool(ctx context.Context, cl client.Client, lsc *v1alpha1.DRBDOperatorStoragePool) error {
	err := cl.Update(ctx, lsc)
	if err != nil {
		return err
	}
	return nil
}

func GetDRBDOperatorStoragePool(ctx context.Context, cl client.Client, namespace, name string) (*v1alpha1.DRBDOperatorStoragePool, error) {
	obj := &v1alpha1.DRBDOperatorStoragePool{}
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

func GetAndValidateVolumeGroups(ctx context.Context, cl client.Client, namespace, lvmType string, drbdspLVMVolumeGroups []v1alpha1.DRBDStoragePoolLVMVolumeGroups) (bool, string, map[string]v1alpha1.LvmVolumeGroup) {
	var lvmVolumeGroupName string
	var nodeName string
	nodesWithlvmVolumeGroups := make(map[string]string)
	invalidLvmVolumeGroups := make(map[string]string)
	lvmVolumeGroupsNames := make(map[string]bool)
	lvmVolumeGroups := make(map[string]v1alpha1.LvmVolumeGroup)

	for _, g := range drbdspLVMVolumeGroups {
		lvmVolumeGroupName = g.Name

		if lvmVolumeGroupsNames[lvmVolumeGroupName] {
			//UpdateMapValue(invalidLvmVolumeGroups, lvmVolumeGroupName, fmt.Sprintf("LvmVolumeGroup name is not unique, %v", lvmVolumeGroupsNames[lvmVolumeGroupName]))
			invalidLvmVolumeGroups[lvmVolumeGroupName] = "LvmVolumeGroup name is not unique"
			continue
		}
		lvmVolumeGroupsNames[lvmVolumeGroupName] = true

		lvmVolumeGroup, err := GetLvmVolumeGroup(ctx, cl, namespace, lvmVolumeGroupName)
		if err != nil {
			UpdateMapValue(invalidLvmVolumeGroups, lvmVolumeGroupName, fmt.Sprintf("Error getting LVMVolumeGroup: %s", err.Error()))
			continue
		}

		if lvmVolumeGroup.Spec.Type != LVMVGTypeLocal {
			UpdateMapValue(invalidLvmVolumeGroups, lvmVolumeGroupName, fmt.Sprintf("LvmVolumeGroup type is not %s", LVMVGTypeLocal))
			continue
		}

		if len(lvmVolumeGroup.Status.Nodes) != 1 {
			UpdateMapValue(invalidLvmVolumeGroups, lvmVolumeGroupName, "LvmVolumeGroup has more than one node in status.nodes. LvmVolumeGroup for LINSTOR Storage Pool must to have only one node")
			continue
		}

		nodeName = lvmVolumeGroup.Status.Nodes[0].Name
		if value, ok := nodesWithlvmVolumeGroups[nodeName]; ok {
			UpdateMapValue(invalidLvmVolumeGroups, lvmVolumeGroupName, fmt.Sprintf("This LvmVolumeGroup have same node %s as LvmVolumeGroup with name: %s. LINSTOR Storage Pool is allowed to have only one LvmVolumeGroup per node", nodeName, value))
		}

		switch lvmType {
		case TypeLVMThin:
			if len(g.ThinPoolName) == 0 {
				UpdateMapValue(invalidLvmVolumeGroups, lvmVolumeGroupName, fmt.Sprintf("type %s but ThinPoolName is not set", TypeLVMThin))
				break
			}
			found := false
			for _, thinPool := range lvmVolumeGroup.Spec.ThinPools {
				if g.ThinPoolName == thinPool.Name {
					found = true
					break
				}
			}
			if !found {
				UpdateMapValue(invalidLvmVolumeGroups, lvmVolumeGroupName, fmt.Sprintf("ThinPoolName %s is not found in Spec.ThinPools of LvmVolumeGroup %s", g.ThinPoolName, lvmVolumeGroupName))
			}
		case TypeLVM:
			if len(g.ThinPoolName) != 0 {
				UpdateMapValue(invalidLvmVolumeGroups, lvmVolumeGroupName, fmt.Sprintf("type %s but ThinPoolName is set", TypeLVM))
			}
		}

		nodesWithlvmVolumeGroups[nodeName] = lvmVolumeGroupName
		lvmVolumeGroups[lvmVolumeGroupName] = *lvmVolumeGroup
	}

	if len(invalidLvmVolumeGroups) > 0 {
		msg := GetOrderedMapValuesAsString(invalidLvmVolumeGroups)
		return false, msg, nil
	}

	return true, "", lvmVolumeGroups
}

func UpdateMapValue(m map[string]string, key string, additionalValue string) {
	if oldValue, ok := m[key]; ok {
		m[key] = fmt.Sprintf("%s. Also: %s", oldValue, additionalValue)
	} else {
		m[key] = additionalValue
	}
}

func GetOrderedMapValuesAsString(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k) // TODO: change append
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, k := range keys {
		v := m[k]
		fmt.Fprintf(&buf, "%s: %s\n", k, v)
	}
	return buf.String()
}
