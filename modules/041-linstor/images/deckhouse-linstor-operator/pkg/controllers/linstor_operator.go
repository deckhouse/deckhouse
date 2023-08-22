package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	LinstorControllerName = "linstor-change-request-controller"
)

func NewLinstorOperator(
	ctx context.Context,
	mgr manager.Manager,
	log logr.Logger,
) (controller.Controller, error) {

	c, err := controller.New(LinstorControllerName, mgr, controller.Options{
		Reconciler: reconcile.Func(func(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
			return reconcile.Result{}, nil
		}),
	})

	err = c.Watch(
		source.Kind(mgr.GetCache(), &v1.Node{}),
		handler.Funcs{
			CreateFunc: func(ctx context.Context, event event.CreateEvent, limitingInterface workqueue.RateLimitingInterface) {
				fmt.Println("CREATE NODE")
			},
		},
	)

	if err != nil {
		return nil, err
	}
	return c, err
}
