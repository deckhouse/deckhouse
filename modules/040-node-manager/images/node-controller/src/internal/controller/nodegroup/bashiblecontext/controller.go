/*
Copyright 2026 Flant JSC

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

package bashiblecontext

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
	"github.com/deckhouse/node-controller/internal/register"
)

const resyncInterval = 10 * time.Minute

func init() {
	register.RegisterController("bashible-context", &v1.NodeGroup{}, &Controller{})
}

type Controller struct {
	register.Base
	apiReader client.Reader
	clientset kubernetes.Interface
}

func (c *Controller) Setup(mgr ctrl.Manager) error {
	c.apiReader = mgr.GetAPIReader()
	cs, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}
	c.clientset = cs
	return nil
}

var assembleRequest = []reconcile.Request{{NamespacedName: types.NamespacedName{Name: "assemble"}}}

func (c *Controller) SetupWatches(w register.Watcher) {
	enqueue := handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
		return assembleRequest
	})
	w.Watches(&corev1.Secret{}, enqueue, builder.WithPredicates(inNamespaces(kubeSystemNS, cloudInstanceManagerNS)))
	w.Watches(&corev1.ConfigMap{}, enqueue, builder.WithPredicates(inNamespaces(kubeSystemNS, versionInfoCMNS)))
	w.Watches(&corev1.Service{}, enqueue, builder.WithPredicates(inNamespaces(kubeSystemNS)))
	w.Watches(&corev1.Pod{}, enqueue, builder.WithPredicates(inNamespaces(kubeSystemNS)))
}

func inNamespaces(namespaces ...string) predicate.Predicate {
	set := make(map[string]bool, len(namespaces))
	for _, ns := range namespaces {
		set[ns] = true
	}
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return set[obj.GetNamespace()]
	})
}

func (c *Controller) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if err := c.ensureCertificate(ctx, logger); err != nil {
		logger.Error(err, "failed to ensure kubernetes-api-proxy discovery certificate")
		return ctrl.Result{}, err
	}

	r := &Reconciler{
		Client:        c.Client,
		Context:       &Service{Client: c.Client, Reader: c.apiReader},
		DerivedStatus: &derived_status.Service{Client: c.Client, Reader: c.apiReader},
	}
	if err := r.Assemble(ctx); err != nil {
		logger.Error(err, "failed to assemble bashible-apiserver-context")
		return ctrl.Result{}, err
	}
	logger.Info("assembled bashible-apiserver-context")
	return ctrl.Result{RequeueAfter: resyncInterval}, nil
}
