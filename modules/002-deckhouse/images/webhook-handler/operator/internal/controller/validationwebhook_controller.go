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
	"path/filepath"
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

const (
	validationHooksBaseDir          = "hooks"
	validationWebhooksSubDir        = "webhooks"
	validatingSubDir                = "validating"
	validationWebhookFileExt        = ".py"
	validationWebhookDirectoryPerms = 0755
	validationWebhookFilePerms      = 0755
)

// ValidationWebhookReconciler reconciles a ValidationWebhook object
type ValidationWebhookReconciler struct {
	isReloadShellNeed *atomic.Bool
	client            client.Client
	scheme            *runtime.Scheme
	logger            *log.Logger
	pythonTemplate    string
}

// NewValidationWebhookReconciler creates a new ValidationWebhookReconciler.
// The isReloadShellNeed flag is shared between reconcilers to signal shell-operator reload.
func NewValidationWebhookReconciler(
	k8sClient client.Client,
	scheme *runtime.Scheme,
	logger *log.Logger,
	pythonTemplate string,
	isReloadShellNeed *atomic.Bool,
) *ValidationWebhookReconciler {
	return &ValidationWebhookReconciler{
		isReloadShellNeed: isReloadShellNeed,
		client:            k8sClient,
		scheme:            scheme,
		logger:            logger.Named("validation-webhook"),
		pythonTemplate:    pythonTemplate,
	}
}

// webhookDir returns the directory path for a webhook's files.
// Example: hooks/deckhouse/webhooks/validating
func (r *ValidationWebhookReconciler) webhookDir(webhookName string) string {
	return filepath.Join(validationHooksBaseDir, webhookName, validationWebhooksSubDir, validatingSubDir)
}

// webhookFilePath returns the full path to a webhook's Python file.
// Example: hooks/deckhouse/webhooks/validating/deckhouse.py
func (r *ValidationWebhookReconciler) webhookFilePath(webhookName string) string {
	return filepath.Join(r.webhookDir(webhookName), webhookName+validationWebhookFileExt)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *ValidationWebhookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var res ctrl.Result

	r.logger.Debug("validating webhook processing started", slog.String("resource_name", req.Name))
	defer func() {
		r.logger.Debug("validating webhook processing complete", slog.String("resource_name", req.Name), slog.Any("reconcile_result", res))
	}()

	webhook := new(deckhouseiov1alpha1.ValidationWebhook)
	err := r.client.Get(ctx, req.NamespacedName, webhook)
	if err != nil {
		r.logger.Warn("error get resource", slog.String("name", req.Name))
		// resource may no longer exist, in which case we stop
		// processing.
		if apierrors.IsNotFound(err) {
			return res, nil
		}

		r.logger.Debug("get validating webhook", log.Err(err))

		return res, err
	}

	// resource marked as "to delete"
	r.logger.Debug("debug deletion timestamp", slog.Any("timestamp", webhook.DeletionTimestamp))
	if !webhook.DeletionTimestamp.IsZero() {
		r.logger.Debug("validating webhook deletion", slog.String("deletion_timestamp", webhook.DeletionTimestamp.String()))

		// TODO: retries
		res, err := r.handleDeleteValidatingWebhook(ctx, webhook)
		if err != nil {
			r.logger.Warn("delete validating webhook", log.Err(err))

			return res, err
		}
		return res, nil
	}

	res, err = r.handleProcessValidatingWebhook(ctx, webhook)
	if err != nil {
		r.logger.Warn("process validating webhook", log.Err(err))

		return res, err
	}

	return res, nil
}

func (r *ValidationWebhookReconciler) handleProcessValidatingWebhook(ctx context.Context, vwh *deckhouseiov1alpha1.ValidationWebhook) (ctrl.Result, error) {
	var res ctrl.Result

	webhookDir := r.webhookDir(vwh.Name)
	if err := os.MkdirAll(webhookDir, validationWebhookDirectoryPerms); err != nil {
		r.logger.Error("failed to create directory", slog.String("path", webhookDir), log.Err(err))
		return res, fmt.Errorf("create dir %s: %w", webhookDir, err)
	}

	buf, err := templater.RenderValidationTemplate(r.pythonTemplate, vwh)
	if err != nil {
		return res, fmt.Errorf("render template: %w", err)
	}

	webhookFile := r.webhookFilePath(vwh.Name)
	if err := os.WriteFile(webhookFile, buf.Bytes(), validationWebhookFilePerms); err != nil {
		r.logger.Error("failed to write webhook file", slog.String("path", webhookFile), log.Err(err))
		return res, fmt.Errorf("write file %s: %w", webhookFile, err)
	}

	r.isReloadShellNeed.Store(true)

	// add finalizer
	if !controllerutil.ContainsFinalizer(vwh, deckhouseiov1alpha1.ValidationWebhookFinalizer) {
		r.logger.Debug("add finalizer")
		controllerutil.AddFinalizer(vwh, deckhouseiov1alpha1.ValidationWebhookFinalizer)

		if err := r.client.Update(ctx, vwh); err != nil {
			if removeErr := os.Remove(webhookFile); removeErr != nil {
				r.logger.Warn("failed to cleanup webhook file", log.Err(removeErr))
			}
			return res, fmt.Errorf("add finalizer: %w", err)
		}
	}

	return res, nil
}

func (r *ValidationWebhookReconciler) handleDeleteValidatingWebhook(ctx context.Context, vh *deckhouseiov1alpha1.ValidationWebhook) (ctrl.Result, error) {
	var res ctrl.Result

	webhookFile := r.webhookFilePath(vh.Name)
	if err := os.Remove(webhookFile); err != nil && !os.IsNotExist(err) {
		return res, fmt.Errorf("delete webhook file %s: %w", webhookFile, err)
	}

	r.isReloadShellNeed.Store(true)

	// remove finalizer
	if controllerutil.ContainsFinalizer(vh, deckhouseiov1alpha1.ValidationWebhookFinalizer) {
		r.logger.Debug("remove finalizer")
		controllerutil.RemoveFinalizer(vh, deckhouseiov1alpha1.ValidationWebhookFinalizer)

		if err := r.client.Update(ctx, vh); err != nil {
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
