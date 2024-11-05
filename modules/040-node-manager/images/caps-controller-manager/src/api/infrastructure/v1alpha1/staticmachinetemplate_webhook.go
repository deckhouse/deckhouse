/*
Copyright 2024 Flant JSC

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

package v1alpha1

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var staticmachinetemplatelog = log.Log.WithName("staticmachinetemplate-webhook")

type StaticMachineTemplateWebhook struct {
	decoder *admission.Decoder
}

func (r *StaticMachineTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	mw := &StaticMachineTemplateWebhook{}
	dec := admission.NewDecoder(mgr.GetScheme())
	mw.InjectDecoder(dec)
	mgr.GetWebhookServer().Register("/validate-infrastructure-cluster-x-k8s-io-v1alpha1-staticmachinetemplate", &webhook.Admission{Handler: mw})
	return nil
}

func (w *StaticMachineTemplateWebhook) InjectDecoder(d *admission.Decoder) error {
	w.decoder = d
	return nil
}

func (w *StaticMachineTemplateWebhook) Handle(ctx context.Context, req admission.Request) admission.Response {
	staticmachinetemplatelog.Info("handle", "operation", req.Operation, "name", req.Name)

	var newObj StaticMachineTemplate
	if err := w.decoder.Decode(req, &newObj); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	var oldObj StaticMachineTemplate
	if req.Operation == admissionv1.Update {
		if err := w.decoder.DecodeRaw(req.OldObject, &oldObj); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	}

	isDeckhouse := isReqFromDeckhouse(req.UserInfo)
	switch req.Operation {
	case admissionv1.Create:
		return w.handleCreate(ctx, newObj)
	case admissionv1.Update:
		return w.handleUpdate(newObj, oldObj, isDeckhouse)
	case admissionv1.Delete:
		return w.handleDelete(ctx, oldObj)
	default:
		return admission.Allowed("operation not handled")
	}
}

func isReqFromDeckhouse(userInfo authenticationv1.UserInfo) bool {
	expectedUsername := "system:serviceaccount:d8-system:deckhouse"
	//debug
	fmt.Printf("userInfo.Username %s", userInfo.Username)
	return userInfo.Username == expectedUsername
}

func (w *StaticMachineTemplateWebhook) handleUpdate(newObj, oldObj StaticMachineTemplate, isDeckhouse bool) admission.Response {

	if !reflect.DeepEqual(newObj.Spec, oldObj.Spec) && !isDeckhouse {
		return admission.Denied("only deckhouse service account can update StaticMachineTemplate.Spec")
	}
	return admission.Allowed("update allowed")
}

func (w *StaticMachineTemplateWebhook) handleDelete(ctx context.Context, oldObj StaticMachineTemplate) admission.Response {
	return admission.Allowed("delete allowed")
}

func (w *StaticMachineTemplateWebhook) handleCreate(ctx context.Context, newObj StaticMachineTemplate) admission.Response {
	return admission.Allowed("create allowed")
}
