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
	"errors"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"
	kjson "sigs.k8s.io/json"
)

const (
	// maximum number of operations a single json patch may contain.
	maxJSONPatchOperations = 10000

	manager = "local validator patcher"
)

// based on https://github.com/kubernetes/apiserver/blob/v0.29.8/pkg/endpoints/handlers/patch.go
func patch(input runtime.Object, patchType types.PatchType, patch []byte, scheme *runtime.Scheme) (runtime.Object, error) {
	var mechanism interface {
		applyPatchToCurrentObject(currentObject runtime.Object) (runtime.Object, error)
	}

	typeConverter := managedfields.NewDeducedTypeConverter()
	fieldManager, err := managedfields.NewDefaultCRDFieldManager(
		typeConverter,
		scheme,
		scheme,
		scheme,
		input.GetObjectKind().GroupVersionKind(),
		input.GetObjectKind().GroupVersionKind().GroupVersion(),
		"",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("new default file manager: %w", err)
	}

	switch patchType {
	case types.JSONPatchType, types.MergePatchType:
		mechanism = &jsonPatcher{
			patchType:    patchType,
			patch:        patch,
			fieldManager: fieldManager,
			scheme:       scheme,
		}
	case types.StrategicMergePatchType:
		schemaReferenceObj, err := scheme.UnsafeConvertToVersion(input, input.GetObjectKind().GroupVersionKind().GroupVersion())
		if err != nil {
			return nil, err
		}
		mechanism = &smpPatcher{
			patch:              patch,
			schemaReferenceObj: schemaReferenceObj,
			fieldManager:       fieldManager,
			scheme:             scheme,
		}
	case types.ApplyPatchType: // TODO: rename to types.ApplyYAMLPatchType:
		mechanism = &applyPatcher{
			fieldManager: fieldManager,
			patch:        patch,
		}
	// TODO: case types.ApplyCBORPatchType:
	default:
		return nil, fmt.Errorf("%v: unimplemented patch type", patchType)
	}

	return mechanism.applyPatchToCurrentObject(input.DeepCopyObject())
}

type jsonPatcher struct {
	patchType    types.PatchType
	patch        []byte
	fieldManager *managedfields.FieldManager
	scheme       *runtime.Scheme
}

func (p *jsonPatcher) applyPatchToCurrentObject(currentObject runtime.Object) (runtime.Object, error) {
	groupKind := currentObject.GetObjectKind().GroupVersionKind().GroupKind()
	codec := serializer.NewCodecFactory(p.scheme).LegacyCodec(p.scheme.VersionsForGroupKind(groupKind)...)
	// Encode will convert & return a versioned object in JSON.
	currentObjJS, err := runtime.Encode(codec, currentObject)
	if err != nil {
		return nil, err
	}

	// Apply the patch.
	patchedObjJS, appliedStrictErrs, err := p.applyJSPatch(currentObjJS)
	if err != nil {
		return nil, err
	}

	// Construct the resulting typed, unversioned object.
	objToUpdate := currentObject.DeepCopyObject()
	if err := runtime.DecodeInto(codec, patchedObjJS, objToUpdate); err != nil {
		strictError, isStrictError := runtime.AsStrictDecodingError(err)
		if !isStrictError {
			// disregard any appliedStrictErrs, because it's an incomplete
			// list of strict errors given that we don't know what fields were
			// unknown because DecodeInto failed. Non-strict errors trump in this case.
			return nil, field.Invalid(field.NewPath("patch"), string(patchedObjJS), err.Error())
		}

		strictDecodingError := runtime.NewStrictDecodingError(append(appliedStrictErrs, strictError.Errors()...))
		return nil, field.Invalid(field.NewPath("patch"), string(patchedObjJS), strictDecodingError.Error())
	} else if len(appliedStrictErrs) > 0 {
		return nil, field.Invalid(field.NewPath("patch"), string(patchedObjJS), runtime.NewStrictDecodingError(appliedStrictErrs).Error())
	}

	objToUpdate = p.fieldManager.UpdateNoErrors(currentObject, objToUpdate, manager)
	return objToUpdate, nil
}

type jsonPatchOp struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	From  string      `json:"from"`
	Value interface{} `json:"value"`
}

// applyJSPatch applies the patch. Input and output objects must both have
// the external version, since that is what the patch must have been constructed against.
func (p *jsonPatcher) applyJSPatch(versionedJS []byte) ([]byte, []error, error) {
	switch p.patchType {
	case types.JSONPatchType:
		var v []jsonPatchOp
		strictErrors, err := kjson.UnmarshalStrict(p.patch, &v)
		if err != nil {
			return nil, nil, fmt.Errorf("error decoding patch: %w", err)
		}

		for i, e := range strictErrors {
			strictErrors[i] = fmt.Errorf("json patch %w", e)
		}

		patchObj, err := jsonpatch.DecodePatch(p.patch)
		if err != nil {
			return nil, nil, err
		}

		if len(patchObj) > maxJSONPatchOperations {
			return nil, nil, fmt.Errorf(
				"the allowed maximum operations in a JSON patch is %d, got %d",
				maxJSONPatchOperations,
				len(patchObj),
			)
		}
		patchedJS, err := patchObj.Apply(versionedJS)
		if err != nil {
			return nil, nil, err
		}
		return patchedJS, strictErrors, nil
	case types.MergePatchType:
		v := map[string]interface{}{}
		strictErrors, err := kjson.UnmarshalStrict(p.patch, &v)
		if err != nil {
			return nil, nil, fmt.Errorf("error decoding patch: %w", err)
		}

		patchedJS, err := jsonpatch.MergePatch(versionedJS, p.patch)
		if errors.Is(err, jsonpatch.ErrBadJSONPatch) {
			return nil, nil, err
		}
		return patchedJS, strictErrors, err
	default:
		// only here as a safety net - go-restful filters content-type
		return nil, nil, fmt.Errorf("unknown Content-Type header for patch: %v", p.patchType)
	}
}

type smpPatcher struct {
	patch              []byte
	schemaReferenceObj runtime.Object
	fieldManager       *managedfields.FieldManager
	scheme             *runtime.Scheme
}

func (p *smpPatcher) applyPatchToCurrentObject(currentObject runtime.Object) (runtime.Object, error) {
	currentVersionedObject := currentObject.DeepCopyObject()
	versionedObjToUpdate := currentObject.DeepCopyObject()
	newObj := currentObject.DeepCopyObject()
	if err := strategicPatchObject(p.scheme, currentVersionedObject, p.patch, versionedObjToUpdate, p.schemaReferenceObj); err != nil {
		return nil, err
	}

	newObj = p.fieldManager.UpdateNoErrors(currentObject, newObj, manager)
	return newObj, nil
}

type applyPatcher struct {
	patch        []byte
	options      *metav1.PatchOptions
	fieldManager *managedfields.FieldManager
}

func (p *applyPatcher) applyPatchToCurrentObject(obj runtime.Object) (runtime.Object, error) {
	force := false
	if p.options.Force != nil {
		force = *p.options.Force
	}
	if p.fieldManager == nil {
		panic("FieldManager must be installed to run apply")
	}

	patchObj := &unstructured.Unstructured{Object: map[string]interface{}{}}
	if err := yaml.Unmarshal(p.patch, &patchObj.Object); err != nil {
		return nil, fmt.Errorf("error decoding YAML: %w", err)
	}

	obj, err := p.fieldManager.Apply(obj, patchObj, p.options.FieldManager, force)
	if err != nil {
		return obj, err
	}

	// TODO: spawn something to track deciding whether a fieldValidation=Strict
	// fatal error should return before an error from the apply operation
	if err = yaml.UnmarshalStrict(p.patch, &map[string]interface{}{}); err != nil {
		return nil, fmt.Errorf("error strict decoding YAML: %v", err)
	}

	return obj, nil
}

// strategicPatchObject applies a strategic merge patch of `patchBytes` to
// `originalObject` and stores the result in `objToUpdate`.
// It additionally returns the map[string]interface{} representation of the
// `originalObject` and `patchBytes`.
// NOTE: Both `originalObject` and `objToUpdate` are supposed to be versioned.
func strategicPatchObject(
	defaulter runtime.ObjectDefaulter,
	originalObject runtime.Object,
	patchBytes []byte,
	objToUpdate runtime.Object,
	schemaReferenceObj runtime.Object,
) error {
	originalObjMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(originalObject)
	if err != nil {
		return err
	}

	patchMap := make(map[string]interface{})
	var strictErrs []error
	strictErrs, err = kjson.UnmarshalStrict(patchBytes, &patchMap)
	if err != nil {
		return err
	}

	return applyPatchToObject(defaulter, originalObjMap, patchMap, objToUpdate, schemaReferenceObj, strictErrs)
}

// applyPatchToObject applies a strategic merge patch of <patchMap> to
// <originalMap> and stores the result in <objToUpdate>.
// NOTE: <objToUpdate> must be a versioned object.
func applyPatchToObject(
	defaulter runtime.ObjectDefaulter,
	originalMap map[string]interface{},
	patchMap map[string]interface{},
	objToUpdate runtime.Object,
	schemaReferenceObj runtime.Object,
	strictErrs []error,
) error {
	patchedObjMap, err := strategicpatch.StrategicMergeMapPatch(originalMap, patchMap, schemaReferenceObj)
	if err != nil {
		return err
	}

	// Rather than serialize the patched map to JSON, then decode it to an object, we go directly from a map to an object
	converter := runtime.DefaultUnstructuredConverter
	returnUnknownFields := true
	if err := converter.FromUnstructuredWithValidation(patchedObjMap, objToUpdate, returnUnknownFields); err != nil {
		strictError, isStrictError := runtime.AsStrictDecodingError(err)
		if !isStrictError {
			// disregard any sttrictErrs, because it's an incomplete
			// list of strict errors given that we don't know what fields were
			// unknown because StrategicMergeMapPatch failed.
			// Non-strict errors trump in this case.
			return field.Invalid(field.NewPath("patch"), fmt.Sprintf("%+v", patchMap), err.Error())
		}

		strictDecodingError := runtime.NewStrictDecodingError(append(strictErrs, strictError.Errors()...))
		return field.Invalid(field.NewPath("patch"), fmt.Sprintf("%+v", patchMap), strictDecodingError.Error())
	} else if len(strictErrs) > 0 {
		return field.Invalid(field.NewPath("patch"), fmt.Sprintf("%+v", patchMap), runtime.NewStrictDecodingError(strictErrs).Error())
	}

	// Decoding from JSON to a versioned object would apply defaults, so we do the same here
	defaulter.Default(objToUpdate)

	return nil
}
