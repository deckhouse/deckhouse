/*
Copyright 2023 Flant JSC

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

package webhookhandler

import (
	"context"
	"net/http"

	"github.com/deckhouse/deckhouse/modules/900-gost-integrity-controller/images/gost-digest-webhook/src/pkg/validation"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type Handler struct {
	decoder   *admission.Decoder
	validator validation.PodValidator
}

func NewHandler(validator validation.PodValidator) admission.Handler {
	return &Handler{validator: validator, decoder: admission.NewDecoder(runtime.NewScheme())}
}

func (a *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := a.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	err = a.validator.Validate(pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	return admission.Allowed("")
}

func (a *Handler) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}
