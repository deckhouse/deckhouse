// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"

	"github.com/deckhouse/deckhouse/pkg/log"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	deckhouseiov1alpha1 "deckhouse.io/webhook/api/v1alpha1"
	"deckhouse.io/webhook/internal/templater"
)

// ConversionWebhookReconciler reconciles a ConversionWebhook object
type ConversionWebhookReconciler struct {
	// Set to true if shell operator needs to be reloaded
	IsReloadShellNeed *atomic.Bool
	Client            client.Client
	Scheme            *runtime.Scheme
	// Slog logger
	Logger *log.Logger
	// Go template with python conversion webhook
	PythonTemplate string
}

// +kubebuilder:rbac:groups=deckhouse.io,resources=conversionwebhooks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deckhouse.io,resources=conversionwebhooks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=deckhouse.io,resources=conversionwebhooks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *ConversionWebhookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var res ctrl.Result

	r.Logger.Debug("conversion webhook processing started", slog.String("resource_name", req.Name))
	defer func() {
		r.Logger.Debug("conversion webhook processing complete", slog.String("resource_name", req.Name), slog.Any("reconcile_result", res))
	}()

	webhook := new(deckhouseiov1alpha1.ConversionWebhook)
	err := r.Client.Get(ctx, req.NamespacedName, webhook)
	if err != nil {
		r.Logger.Warn("error get resource", slog.String("name", req.Name))
		// resource may no longer exist, in which case we stop
		// processing.
		if apierrors.IsNotFound(err) {
			return res, nil
		}

		r.Logger.Debug("get conversion webhook", log.Err(err))

		return res, err
	}

	// resource marked as "to delete"
	r.Logger.Debug("debug deletion timestamp", slog.Any("timestamp", webhook.DeletionTimestamp))
	if !webhook.DeletionTimestamp.IsZero() {
		r.Logger.Debug("conversion webhook deletion", slog.String("deletion_timestamp", webhook.DeletionTimestamp.String()))

		res, err := r.handleDeleteConversionWebhook(ctx, webhook)
		if err != nil {
			r.Logger.Warn("delete conversion webhook", log.Err(err))

			return res, err
		}
		return res, nil
	}

	res, err = r.handleProcessConversionWebhook(ctx, webhook)
	if err != nil {
		r.Logger.Warn("process conversion webhook", log.Err(err))

		return res, err
	}

	return res, nil
}

func (r *ConversionWebhookReconciler) handleProcessConversionWebhook(ctx context.Context, cwh *deckhouseiov1alpha1.ConversionWebhook) (ctrl.Result, error) {
	var res ctrl.Result

	_, _ = ctx, cwh

	// example path: hooks/002-deckhouse/webhooks/conversion/
	webhookDir := "hooks/" + cwh.Name + "/webhooks/conversion/"
	err := os.MkdirAll(webhookDir, 0777)
	if err != nil {
		log.Error("create dir: %w", err)
		res.Requeue = true
		return res, fmt.Errorf("create dir %s: %w", webhookDir, err)
	}

	buf, err := templater.RenderConversionTemplate(r.PythonTemplate, cwh)
	if err != nil {
		return res, fmt.Errorf("render template: %w", err)
	}

	// filepath example: hooks/002-deckhouse/webhooks/conversion/deckhouse.py
	err = os.WriteFile(webhookDir+cwh.Name+".py", buf.Bytes(), 0755)
	if err != nil {
		log.Error("create file: %w", err)
	}

	r.IsReloadShellNeed.Store(true)

	// add finalizer
	if !controllerutil.ContainsFinalizer(cwh, deckhouseiov1alpha1.ConversionWebhookFinalizer) {
		r.Logger.Debug("add finalizer")
		controllerutil.AddFinalizer(cwh, deckhouseiov1alpha1.ConversionWebhookFinalizer)

		err = r.Client.Update(ctx, cwh)
		if err != nil {
			res.Requeue = true
			os.Remove(webhookDir + cwh.Name + ".py")
			return res, fmt.Errorf("add finalizer: %w", err)
		}
	}

	return res, nil
}

func (r *ConversionWebhookReconciler) handleDeleteConversionWebhook(ctx context.Context, cwh *deckhouseiov1alpha1.ConversionWebhook) (ctrl.Result, error) {
	var res ctrl.Result

	_, _ = ctx, cwh

	// filepath example: hooks/deckhouse/webhooks/conversion/deckhouse.py
	filePath := "hooks/" + cwh.Name + "/webhooks/conversion/" + cwh.Name + ".py"
	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		res.Requeue = true
		return res, fmt.Errorf("error delete webhook file %s: %w", filePath, err)
	}

	r.IsReloadShellNeed.Store(true)

	// remove finalizer
	if controllerutil.ContainsFinalizer(cwh, deckhouseiov1alpha1.ConversionWebhookFinalizer) {
		r.Logger.Debug("remove finalizer")
		controllerutil.RemoveFinalizer(cwh, deckhouseiov1alpha1.ConversionWebhookFinalizer)

		err = r.Client.Update(ctx, cwh)
		if err != nil {
			res.Requeue = true
			return res, fmt.Errorf("remove finalizer for %s: %w", cwh.Name, err)
		}
	}

	return res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConversionWebhookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&deckhouseiov1alpha1.ConversionWebhook{}).
		Named("conversionwebhook").
		Complete(r)
}
