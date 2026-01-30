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
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/deckhouse/deckhouse/pkg/log"

	deckhouseiov1alpha1 "deckhouse.io/webhook/api/v1alpha1"
	"deckhouse.io/webhook/internal/templater"
)

// ValidationWebhookReconciler reconciles a ValidationWebhook object
type ValidationWebhookReconciler struct {
	// Set to true if shell operator needs to be reloaded
	IsReloadShellNeed *atomic.Bool
	Client            client.Client
	Scheme            *runtime.Scheme
	// Slog logger
	Logger *log.Logger
	// Go template with python validating webhook
	PythonTemplate string
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
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
		r.Logger.Warn("error get resource", slog.String("name", req.Name))
		// resource may no longer exist, in which case we stop
		// processing.
		if apierrors.IsNotFound(err) {
			return res, nil
		}

		r.Logger.Debug("get validating webhook", log.Err(err))

		return res, err
	}

	// resource marked as "to delete"
	r.Logger.Debug("debug deletion timestamp", slog.Any("timestamp", webhook.DeletionTimestamp))
	if !webhook.DeletionTimestamp.IsZero() {
		r.Logger.Debug("validating webhook deletion", slog.String("deletion_timestamp", webhook.DeletionTimestamp.String()))

		// TODO: retries
		res, err := r.handleDeleteValidatingWebhook(ctx, webhook)
		if err != nil {
			r.Logger.Warn("delete validating webhook", log.Err(err))

			return res, err
		}
		return res, nil
	}

	res, err = r.handleProcessValidatingWebhook(ctx, webhook)
	if err != nil {
		r.Logger.Warn("process validating webhook", log.Err(err))

		return res, err
	}

	return res, nil
}

func (r *ValidationWebhookReconciler) handleProcessValidatingWebhook(ctx context.Context, vh *deckhouseiov1alpha1.ValidationWebhook) (ctrl.Result, error) {
	var res ctrl.Result

	_, _ = ctx, vh

	// example path: hooks/002-deckhouse/webhooks/validating/
	webhookDir := "hooks/" + vh.Name + "/webhooks/validating/"
	err := os.MkdirAll(webhookDir, 0777)
	if err != nil {
		log.Error("create dir: %w", err)
		res.Requeue = true
		return res, fmt.Errorf("create dir %s: %w", webhookDir, err)
	}

	buf, err := templater.RenderValidationTemplate(r.PythonTemplate, vh)
	if err != nil {
		return res, fmt.Errorf("render template: %w", err)
	}

	// filepath example: hooks/deckhouse/webhooks/validating/deckhouse.py
	err = os.WriteFile(webhookDir+vh.Name+".py", buf.Bytes(), 0755)
	if err != nil {
		log.Error("create file: %w", err)
	}

	r.IsReloadShellNeed.Store(true)

	// add finalizer
	if !controllerutil.ContainsFinalizer(vh, deckhouseiov1alpha1.ValidationWebhookFinalizer) {
		r.Logger.Debug("add finalizer")
		controllerutil.AddFinalizer(vh, deckhouseiov1alpha1.ValidationWebhookFinalizer)

		err = r.Client.Update(ctx, vh)
		if err != nil {
			res.Requeue = true
			if removeErr := os.Remove(webhookDir + vh.Name + ".py"); removeErr != nil {
				r.Logger.Warn("failed to cleanup webhook file", log.Err(removeErr))
			}
			return res, fmt.Errorf("add finalizer: %w", err)
		}
	}

	return res, nil
}

func (r *ValidationWebhookReconciler) handleDeleteValidatingWebhook(ctx context.Context, vh *deckhouseiov1alpha1.ValidationWebhook) (ctrl.Result, error) {
	var res ctrl.Result

	_, _ = ctx, vh

	// filepath example: hooks/deckhouse/webhooks/validating/deckhouse.py
	webhookDir := "hooks/" + vh.Name + "/webhooks/validating/"
	err := os.Remove(webhookDir + vh.Name + ".py")
	if err != nil && !os.IsNotExist(err) {
		res.Requeue = true
		return res, fmt.Errorf("error delete webhook file %s: %w", webhookDir+vh.Name+".py", err)
	}

	r.IsReloadShellNeed.Store(true)

	// remove finalizer
	if controllerutil.ContainsFinalizer(vh, deckhouseiov1alpha1.ValidationWebhookFinalizer) {
		r.Logger.Debug("remove finalizer")
		controllerutil.RemoveFinalizer(vh, deckhouseiov1alpha1.ValidationWebhookFinalizer)

		err = r.Client.Update(ctx, vh)
		if err != nil {
			res.Requeue = true
			return res, fmt.Errorf("remove finalizer for %s: %w", vh.Name, err)
		}
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
