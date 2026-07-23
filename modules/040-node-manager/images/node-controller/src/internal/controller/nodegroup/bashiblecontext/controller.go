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
	// lastAssemble implements the debounce. Every event maps to the single fixed "assemble"
	// request key, and the workqueue never hands one key to two workers at once, so the
	// field is only ever touched sequentially — no synchronization needed.
	lastAssemble time.Time
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

// ForPredicates: the assembled context depends on NodeGroup specs only, so status writes
// and finalizer patches must not re-run the whole assembly (each run derives every
// NodeGroup — during a burst the unfiltered events multiplied into hundreds of passes).
func (c *Controller) ForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.Or(
		predicate.GenerationChangedPredicate{},
		predicate.AnnotationChangedPredicate{},
	)}
}

func (c *Controller) SetupWatches(w register.Watcher) {
	enqueue := handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
		return assembleRequest
	})
	// The controller's own output Secret must not feed back into its trigger, otherwise
	// every assembly re-enqueues the next one and the loop free-runs.
	notOwnSecret := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return !(obj.GetNamespace() == cloudInstanceManagerNS && obj.GetName() == secretName)
	})
	w.Watches(&corev1.Secret{}, enqueue, builder.WithPredicates(predicate.And(
		inNamespaces(kubeSystemNS, cloudInstanceManagerNS), notOwnSecret)))
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

// assembleDebounce coalesces context assemblies: every write of the output Secret makes
// bashible-apiserver re-render every bashible step for every NodeGroup (an expensive full
// rebuild), so a burst of NodeGroup changes must collapse into one assembly per window
// instead of one per event.
const assembleDebounce = 3 * time.Second

func (c *Controller) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if since := time.Since(c.lastAssemble); since < assembleDebounce {
		return ctrl.Result{RequeueAfter: assembleDebounce - since}, nil
	}
	c.lastAssemble = time.Now()

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
