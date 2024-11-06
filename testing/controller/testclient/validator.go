// Copyright 2024 Flant JSC
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

package testclient

import (
	"reflect"

	"github.com/flant/addon-operator/pkg/utils/logger"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/validation/validate"
)

func NewValidator(logger logger.Logger, validators map[schema.GroupVersionKind]validation.SchemaValidator) *Validator {
	return &Validator{
		logger:     logger,
		validators: validators,
	}
}

var _ validation.SchemaValidator = (*Validator)(nil)

type Validator struct {
	logger     logger.Logger
	validators map[schema.GroupVersionKind]validation.SchemaValidator
}

func (v *Validator) Validate(obj any, options ...validation.ValidationOption) *validate.Result {
	runtimeObject, ok := obj.(runtime.Object)
	if !ok {
		v.logger.Debug("unsupported type", "obj: ", reflect.TypeOf(obj))
		return nil
	}

	validator := v.GetValidatorFor(runtimeObject.GetObjectKind())
	if validator == nil {
		return nil
	}

	return validator.Validate(obj, options...)
}

func (v *Validator) ValidateUpdate(newObj, oldObj any, options ...validation.ValidationOption) *validate.Result {
	runtimeObject, ok := newObj.(runtime.Object)
	if !ok {
		v.logger.Debug("unsupported type", "obj: ", reflect.TypeOf(newObj))
		return nil
	}

	validator := v.GetValidatorFor(runtimeObject.GetObjectKind())
	if validator == nil {
		return nil
	}

	return validator.ValidateUpdate(newObj, oldObj, options...)
}

func (v *Validator) GetValidatorFor(kind schema.ObjectKind) validation.SchemaValidator {
	if kind == nil {
		v.logger.Warn("empty object kind")
		return nil
	}

	gvk := kind.GroupVersionKind()
	gvkValidator := v.validators[gvk]
	if gvkValidator == nil {
		v.logger.Debugf("validator for %s not found. Available validators: %s", gvk, keys(v.validators))
		return nil
	}

	return gvkValidator
}

func keys(validators map[schema.GroupVersionKind]validation.SchemaValidator) []string {
	result := make([]string, 0, len(validators))

	for k := range validators {
		result = append(result, k.GroupVersion().String())
	}

	return result
}
