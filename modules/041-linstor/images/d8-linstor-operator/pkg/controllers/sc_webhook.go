package controllers

import (
	"context"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func NewCSValidator(log logr.Logger) *CSValidator {
	return &CSValidator{log: log}
}

type CSValidator struct {
	log logr.Logger
}

func (v *CSValidator) ValidateCreate(_ context.Context, object runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *CSValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	v.log.Info("Сhanges StorageClass")
	return nil, nil
}

func (v *CSValidator) ValidateDelete(_ context.Context, object runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
