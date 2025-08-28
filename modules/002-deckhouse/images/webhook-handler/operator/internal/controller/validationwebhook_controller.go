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
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/template"

	"github.com/deckhouse/deckhouse/pkg/log"
	"gopkg.in/yaml.v3"
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

// toYAML takes an interface, marshals it to yaml, and returns a string. It will
// always return a string, even on marshal error (empty string).
//
// This is designed to be called from a template.
func toYAML(v interface{}) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return strings.TrimSuffix(string(data), "\n")
}

func indent(spaces int, s string) string {
	pad := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
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
	err := os.MkdirAll("hooks/"+vh.Name+"/webhooks/validating/", 0777)
	if err != nil {
		log.Error("create dir: %w", err)
	}

	tplt := `
#!/usr/bin/python3
from typing import Optional

from deckhouse import hook
from dotmap import DotMap
from cryptography import x509
from cryptography.hazmat.backends import default_backend

config = """
configVersion: v1
kubernetesValidating:
- group: main
{{ toYaml .Spec.Webhook | indent 2 }}
kubernetes:
- name: {{ .Name }}
  group: main
{{ toYaml .Spec.Context | indent 2 }}
"""

def main(ctx: hook.Context):
    try:
        # DotMap is a dict with dot notation
        binding_context = DotMap(ctx.binding_context)
        validate(binding_context, ctx.output.validations)
    except Exception as e:
        ctx.output.validations.error(str(e))

{{ .Spec.Handler.Python }}

if __name__ == "__main__":
    hook.run(main, config=config)
`

	tpl, err := template.New("test").Funcs(template.FuncMap{
		"toYaml": toYAML,
		"indent": indent,
	}).Parse(tplt)
	if err != nil {
		return res, fmt.Errorf("template parse: %w", err)
	}

	var buf bytes.Buffer

	err = tpl.Execute(&buf, vh)
	if err != nil {
		return res, fmt.Errorf("template execute: %w", err)
	}
	// log.Info("template", slog.String("template", buf.String()))
	fmt.Println(buf.String())

	err = os.WriteFile("hooks/"+vh.Name+"/webhooks/validating/"+vh.Name+".py", buf.Bytes(), 0755)
	if err != nil {
		log.Error("create file: %w", err)
	}

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
