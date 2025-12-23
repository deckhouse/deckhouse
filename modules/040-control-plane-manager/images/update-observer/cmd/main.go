package main

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func main() {
	var log = ctrl.Log.WithName("update-observer")

	manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		NewCache: func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
			opts.DefaultNamespaces = map[string]cache.Config{
				"kube-system": {},
			}
			return cache.New(config, opts)
		},
	})

	if err != nil {
		log.Error(err, "could not create manager")
		os.Exit(1)
	}

	err = ctrl.NewControllerManagedBy(manager).
		Named("version-observer").
		Watches(
			&corev1.Secret{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(
				GetSecretPredicate(
					"d8-cluster-configuration",
				),
			),
		).
		Complete(&Reconciler{manager.GetClient()})

	if err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "could not start manager")
		os.Exit(1)
	}
}

func GetSecretPredicate(secretName string) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			secret, ok := e.Object.(*corev1.Secret)
			if !ok {
				return false
			}
			return secret.Name == secretName
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			secret, ok := e.ObjectNew.(*corev1.Secret)
			if !ok {
				return false
			}
			return secret.Name == secretName
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			secret, ok := e.Object.(*corev1.Secret)
			if !ok {
				return false
			}
			return secret.Name == secretName
		},

		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

type Reconciler struct {
	client.Client
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	fmt.Println("Reconciling start!")

	p := &corev1.Secret{}
	if err := r.Get(ctx, req.NamespacedName, p); err != nil {
		return reconcile.Result{}, err
	}

	fmt.Println(p.Name)
	return reconcile.Result{}, nil
}
