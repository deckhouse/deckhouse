package controller

import (
	"context"
	"errors"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"st2/api/v1alpha1"
	"strconv"
	"strings"
)

const (
	LinstorStorageClassControllerName = "linstor-storage-class-controller"
	Provisioner                       = "linstor.csi.linbit.com"
	StorageClassKind                  = "StorageClass"
	StorageClassAPIVersion            = "storage.k8s.io/v1"

	LinstorPlacementCount = "linstor.csi.linbit.com/placementCount"
	LinstorStoragePool    = "linstor.csi.linbit.com/storagePool"
	AutoQuorum            = "property.linstor.csi.linbit.com/DrbdOptions/auto-quorum"

	Completed = "Completed"
	Failed    = "Failed"
)

func NewLinstorStorageClass(
	ctx context.Context,
	mgr manager.Manager,
) (controller.Controller, error) {
	cl := mgr.GetClient()
	log := mgr.GetLogger()
	c, err := controller.New(LinstorStorageClassControllerName, mgr, controller.Options{
		Reconciler: reconcile.Func(func(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
			return reconcile.Result{}, nil
		}),
	})

	if err != nil {
		return nil, err
	}

	err = c.Watch(
		source.Kind(mgr.GetCache(), &v1alpha1.LinstorStorageClass{}),
		handler.Funcs{
			CreateFunc: func(ctx context.Context, e event.CreateEvent, q workqueue.RateLimitingInterface) {

				// ----------------------- get LinstorStorageClass ----------------------------------
				log.Info("get LinstorStorageClass " + e.Object.GetName())
				lsc, err1 := getLinstorStorageClass(ctx, cl, e.Object.GetNamespace(), e.Object.GetName())
				if err1 != nil {
					log.Error(err1, "error get LinstorStorageClass ")
					return
				}
				// ------------------------ get LinstorStorageClass ----------------------------------

				if lsc.Status.Phase == Completed {
					log.Info("linstor storage class pass....")
					return
				}

				// ------------------------ get StorageClass -----------------------------------------
				log.Info("get StorageClass " + e.Object.GetName())
				sc, err2 := getStorageClass(ctx, cl, lsc.Namespace, lsc.Name)
				if err2 != nil {
					log.Error(err2, "error get StorageClass")
					if strings.Contains(err2.Error(), "not found") {
						// ------------------------- LinstorStorageClass -> create CreateStorage -------------
						err = createStorageClass(ctx, cl, lsc)
						if err != nil {
							log.Error(err, "error create storage class")
							return
						}

						// New status
						lsc.Status.Phase = Completed
						lsc.Status.Reason = "storage class created"

						err = updateLinstorStorageClass(ctx, cl, lsc)
						if err != nil {
							log.Error(err, "error update linstor storage class")
							return
						}
						// ------------------------- LinstorStorageClass -> create CreateStorage -------------
					}
					return
				}
				// ------------------------ get StorageClass -----------------------------------------

				// ------------------------- Check Provisioner ---------------------------------------
				//todo LinstorStorageClass will created without Errors kubectl  ?
				if sc.Provisioner != Provisioner {
					lsc.Status.Phase = Failed
					lsc.Status.Reason = "error Provisioner "
					log.Error(errors.New("error storage class provisioner "), sc.Provisioner)

					err = updateLinstorStorageClass(ctx, cl, lsc)
					if err != nil {
						log.Error(err, "error update linstor storage class")
						return
					}
					return
				}
				// ------------------------- Check Provisioner --------------------------------------- )

				// ------------------------- compare StorageClass vs  Request LinstorStorageClass ----
				//todo. this field must be filled in the StorageClass
				if sc.Parameters != nil {
					if strconv.Itoa(lsc.Spec.PlacementCount) != sc.Parameters[LinstorPlacementCount] {
						sc.Parameters[LinstorPlacementCount] = strconv.Itoa(lsc.Spec.PlacementCount)
					}

					if lsc.Spec.LinstorStoragePool != sc.Parameters[LinstorStoragePool] {
						sc.Parameters[LinstorStoragePool] = lsc.Spec.LinstorStoragePool
					}

					if lsc.Spec.DrbdOptions.AutoQuorum != sc.Parameters[AutoQuorum] {
						sc.Parameters[AutoQuorum] = lsc.Spec.DrbdOptions.AutoQuorum
					}

					var rp string
					if sc.ReclaimPolicy != nil {
						rp = string(*sc.ReclaimPolicy)
					}

					if lsc.Spec.ReclaimPolicy != rp {
						nrp := v1.PersistentVolumeReclaimPolicy(lsc.Spec.ReclaimPolicy)
						sc.ReclaimPolicy = &nrp
					}

					var vbm string
					if sc.VolumeBindingMode != nil {
						vbm = string(*sc.VolumeBindingMode)
					}

					if lsc.Spec.VolumeBindingMode != vbm {
						nvbm := storagev1.VolumeBindingMode(lsc.Spec.VolumeBindingMode)
						sc.VolumeBindingMode = &nvbm
					}
				}

				// ------------------------- compare StorageClass vs  Request LinstorStorageClass ----

				err = updateStorageClass(ctx, cl, sc)
				if err != nil {
					log.Error(err, "error update  storage class")
					return
				}

				lsc.Status.Phase = Completed
				lsc.Status.Reason = "storage class created / updated"

				// New status
				err = updateLinstorStorageClass(ctx, cl, lsc)
				if err != nil {
					log.Error(err, "error update linstor storage class")
					return
				}
			},
			UpdateFunc: nil,
			DeleteFunc: func(ctx context.Context, e event.DeleteEvent, q workqueue.RateLimitingInterface) {
				err = deleteStorageClass(ctx, cl, e.Object.GetName())
				if err != nil {
					log.Error(err, "error deleting storage class "+e.Object.GetName())
				}
				log.Info("deleted storage class " + e.Object.GetName())
			},
		})
	if err != nil {
		return nil, err
	}
	return c, err
}

func createStorageClass(ctx context.Context, cl client.Client, lsc *v1alpha1.LinstorStorageClass) error {

	rp := v1.PersistentVolumeReclaimPolicy(lsc.Spec.ReclaimPolicy)
	vbm := storagev1.VolumeBindingMode(lsc.Spec.VolumeBindingMode)

	paramets := map[string]string{
		LinstorPlacementCount: strconv.Itoa(lsc.Spec.PlacementCount),
		LinstorStoragePool:    lsc.Spec.LinstorStoragePool,
		AutoQuorum:            lsc.Spec.DrbdOptions.AutoQuorum}

	csObj := storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{
			Kind:       StorageClassKind,
			APIVersion: StorageClassAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            lsc.Name,
			Namespace:       lsc.Namespace,
			OwnerReferences: nil,
			Finalizers:      nil,
			ManagedFields:   nil,
		},
		AllowVolumeExpansion: &lsc.Spec.AllowVolumeExpand,
		Parameters:           paramets,
		Provisioner:          Provisioner,
		ReclaimPolicy:        &rp,
		VolumeBindingMode:    &vbm,
	}

	err := cl.Create(ctx, &csObj)
	if err != nil {
		return err
	}
	return nil
}

func getLinstorStorageClass(ctx context.Context, cl client.Client, namespace, name string) (*v1alpha1.LinstorStorageClass, error) {
	obj := &v1alpha1.LinstorStorageClass{}
	err := cl.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, obj)
	if err != nil {
		return nil, err
	}
	return obj, err
}

func getStorageClass(ctx context.Context, cl client.Client, namespace, name string) (*storagev1.StorageClass, error) {
	obj := &storagev1.StorageClass{}
	err := cl.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func updateLinstorStorageClass(ctx context.Context, cl client.Client, lsc *v1alpha1.LinstorStorageClass) error {
	err := cl.Update(ctx, lsc)
	if err != nil {
		return err
	}
	return nil
}

func updateStorageClass(ctx context.Context, cl client.Client, sc *storagev1.StorageClass) error {
	err := cl.Update(ctx, sc)
	if err != nil {
		return nil
	}
	return nil
}

func deleteStorageClass(ctx context.Context, cl client.Client, name string) error {
	csObject := &storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{
			Kind:       StorageClassKind,
			APIVersion: StorageClassAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	err := cl.Delete(ctx, csObject)
	if err != nil {
		return err
	}
	return nil
}
