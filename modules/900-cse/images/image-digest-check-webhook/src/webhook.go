package main

import (
	"context"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type ImageDigestCheckWebhook struct {
	decoder *admission.Decoder
}

func (a *ImageDigestCheckWebhook) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := a.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	return admission.Response{}
}

func (a *ImageDigestCheckWebhook) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}
