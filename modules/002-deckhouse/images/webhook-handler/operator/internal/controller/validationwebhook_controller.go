/*
Copyright 2025.

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
	"log/slog"
	"os"
	"text/template"

	"github.com/deckhouse/deckhouse/pkg/log"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	deckhouseiov1alpha1 "deckhouse.io/webhook/api/v1alpha1"
)

// ValidationWebhookReconciler reconciles a ValidationWebhook object
type ValidationWebhookReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
	// init logger as in docs builder (watcher)
	Logger *log.Logger
}

// +kubebuilder:rbac:groups=deckhouse.io,resources=validationwebhooks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deckhouse.io,resources=validationwebhooks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=deckhouse.io,resources=validationwebhooks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ValidationWebhook object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *ValidationWebhookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var res ctrl.Result

	r.Logger.Debug("validating webhook processing started", slog.String("resource_name", req.Name))
	defer func() {
		r.Logger.Debug("validating webhook processing complete", slog.String("resource_name", req.Name), slog.Any("reconcile_result", res))
	}()

	webhook := new(deckhouseiov1alpha1.ValidationWebhook)
	err := r.Client.Get(ctx, req.NamespacedName, webhook)
	if err != nil {
		// resource may no longer exist, in which case we stop
		// processing.
		if apierrors.IsNotFound(err) {
			return res, nil
		}

		r.Logger.Debug("get validating webhook", log.Err(err))

		return res, err
	}

	// resource marked as "to delete"
	if !webhook.DeletionTimestamp.IsZero() {
		// TODO: finalizer deletion logic
		r.Logger.Debug("release deletion", slog.String("deletion_timestamp", webhook.DeletionTimestamp.String()))

		res, err := r.handleDeleteValidatingWebhook(ctx, webhook)
		if err != nil {
			r.Logger.Warn("bla bla", log.Err(err))

			return res, err
		}
		return res, nil
	}

	res, err = r.handleProcessValidatingWebhook(ctx, webhook)
	if err != nil {
		r.Logger.Warn("bla bla", log.Err(err))

		return res, err
	}

	return res, nil
}

func (r *ValidationWebhookReconciler) handleProcessValidatingWebhook(ctx context.Context, vh *deckhouseiov1alpha1.ValidationWebhook) (ctrl.Result, error) {
	var res ctrl.Result

	_, _ = ctx, vh

	// logic for processing validation webhook
	// 1) MkdirAll to folder
	// 2) Open template (maybe once at startup?)
	// 3) Render template
	// 4) Write to file (add finalizer)
	// 5) write finalizer
	// 6) kill shell-operator binary (we can start shell operator as library too?)

	// hooks/002-deckhouse/webhooks/validating
	os.MkdirAll("/hooks/"+vh.Name+"/webhooks/validating/", 0777)

	templateFile := "templates/webhook.tpl"
	tpl, err := template.ParseFiles(templateFile)
	if err != nil {
		// TODO: do something
	}

	tpl.Execute(os.Stdout, vh)

	// add finalizer
	if !controllerutil.ContainsFinalizer(vh, "some finalizer") {
		controllerutil.AddFinalizer(vh, "some finalizer")
	}

	return res, nil
}

func (r *ValidationWebhookReconciler) handleDeleteValidatingWebhook(ctx context.Context, vh *deckhouseiov1alpha1.ValidationWebhook) (ctrl.Result, error) {
	var res ctrl.Result

	_, _ = ctx, vh

	// logic for processing validation webhook deletion
	// 1) delete file

	// add finalizer
	if controllerutil.ContainsFinalizer(vh, "some finalizer") {
		controllerutil.RemoveFinalizer(vh, "some finalizer")
	}

	return res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ValidationWebhookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&deckhouseiov1alpha1.ValidationWebhook{}).
		Named("validationwebhook").
		Complete(r)
}
