/*
Copyright 2021 Flant JSC

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

package main

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciler struct {
	client.Client
}

func (r *Reconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	println("reconcile started")

	secret := &corev1.Secret{}

	if err := r.Get(ctx, req.NamespacedName, secret); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	for i := 0; i < 60; i++ {
		select {
		case <-ctx.Done():
			println("context cancelled")
			return ctrl.Result{}, ctx.Err()
		default:
		}
		time.Sleep(time.Second)

		expensiveCalculation()
	}

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}

	secret.Data["finished"] = []byte(time.Now().String())

	err := r.Update(ctx, secret)

	println("reconcile finished")

	return ctrl.Result{}, err
}

func expensiveCalculation() {
	x := 0

	for i := 0; i < 10000000; i++ {
		x += i
	}
	_ = x
}

func (r *Reconciler) SetupWithManager(
	mgr ctrl.Manager,
) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		Complete(r)
}
