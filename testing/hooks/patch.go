package hooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"

	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/hashicorp/go-multierror"
	jsonpatch "gopkg.in/evanphx/json-patch.v4"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

type KubernetesPatch struct {
	ObjectStore object_store.ObjectStore
}

func NewKubernetesPatch(objectStore object_store.ObjectStore) *KubernetesPatch {
	return &KubernetesPatch{ObjectStore: objectStore}
}

func (f *KubernetesPatch) CreateObject(object *unstructured.Unstructured, subresource string) error {
	var newObj unstructured.Unstructured
	newObj.SetUnstructuredContent(object.UnstructuredContent())

	if f.ObjectStore.KubernetesResource(newObj.GetKind(), newObj.GetNamespace(), newObj.GetName()).Exists() {
		return fmt.Errorf("kubernetes Create operation failed: resource %s/%s in ns %s is already exists", newObj.GetKind(), newObj.GetName(), newObj.GetNamespace())
	}

	f.ObjectStore.PutObject(newObj.Object, object_store.NewMetaIndex(newObj.GetKind(), newObj.GetNamespace(), newObj.GetName()))

	return nil
}

func (f *KubernetesPatch) CreateOrUpdateObject(object *unstructured.Unstructured, subresource string) error {
	var newObj unstructured.Unstructured
	newObj.SetUnstructuredContent(object.UnstructuredContent())

	f.ObjectStore.PutObject(newObj.Object, object_store.NewMetaIndex(newObj.GetKind(), newObj.GetNamespace(), newObj.GetName()))

	return nil
}

func (f *KubernetesPatch) FilterObject(filterFunc func(*unstructured.Unstructured) (*unstructured.Unstructured, error), _, kind, namespace, name, _ string) error {
	if filterFunc == nil {
		return errors.New("filterFunc is nil")
	}

	obj, ok := f.ObjectStore.GetObject(object_store.NewMetaIndex(kind, namespace, name))
	if !ok {
		return fmt.Errorf("can't find resource %s/%s/%s to patch", kind, namespace, name)
	}

	filteredObj, err := filterFunc(&unstructured.Unstructured{Object: obj})
	if err != nil {
		return err
	}

	f.ObjectStore.PutObject(filteredObj.UnstructuredContent(), object_store.NewMetaIndex(kind, namespace, name))

	return nil
}

func (f *KubernetesPatch) JQPatchObject(jqPatch, _, kind, namespace, name, _ string) error {
	var (
		objJSON []byte
		err     error
	)
	if obj, ok := f.ObjectStore.GetObject(object_store.NewMetaIndex(kind, namespace, name)); ok {
		objJSON, err = json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON object: %s\n\n%+v", err, obj)
		}
	} else {
		return fmt.Errorf("can't find resource %s/%s/%s to patch", kind, namespace, name)
	}

	stdin := bytes.Buffer{}
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	cmd := exec.Command("jq", "-c", jqPatch)
	cmd.Stdin = &stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	stdin.Write(objJSON)

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run jq: %s\n\n%+v", err, cmd)
	}
	patchedObjJSON := stdout.Bytes()

	var t interface{}
	err = json.Unmarshal(patchedObjJSON, &t)
	if err != nil {
		return err
	}

	var patchedObj unstructured.Unstructured
	patchedObj.SetUnstructuredContent(t.(map[string]interface{}))
	f.ObjectStore.PutObject(patchedObj.Object, object_store.NewMetaIndex(patchedObj.GetKind(), patchedObj.GetNamespace(), patchedObj.GetName()))

	return nil
}

func (f *KubernetesPatch) MergePatchObject(mergePatch []byte, _, kind, namespace, name, _ string) error {
	var objToPatch object_store.KubeObject

	objToPatch, ok := f.ObjectStore.GetObject(object_store.NewMetaIndex(kind, namespace, name))
	if !ok {
		return fmt.Errorf("can't find resource %s/%s/%s to patch", kind, namespace, name)
	}

	objToPatchJSON, err := json.Marshal(objToPatch)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %s\n\n%+v", err, objToPatch)
	}
	if string(objToPatchJSON) == "null" {
		objToPatchJSON = []byte("{}")
	}

	modifiedJSON, err := jsonpatch.MergePatch(objToPatchJSON, mergePatch)
	if err != nil {
		return err
	}

	var modifiedObj map[string]interface{}
	err = json.Unmarshal(modifiedJSON, &modifiedObj)
	if err != nil {
		return err
	}

	f.ObjectStore.PutObject(modifiedObj, object_store.NewMetaIndex(kind, namespace, name))

	return nil
}

func (f *KubernetesPatch) JSONPatchObject(jsonPatch []byte, _, kind, namespace, name, _ string) error {
	var modifiedJSON []byte
	var modifiedObj object_store.KubeObject
	var err error

	objToPatch, ok := f.ObjectStore.GetObject(object_store.NewMetaIndex(kind, namespace, name))
	if !ok {
		return fmt.Errorf("can't find resource %s/%s/%s to patch", kind, namespace, name)
	}

	objectToPatchJSON, err := json.Marshal(objToPatch)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %s\n\n%+v", err, objToPatch)
	}
	if string(objectToPatchJSON) == "null" {
		objectToPatchJSON = []byte("{}")
	}

	var patch jsonpatch.Patch
	patch, err = jsonpatch.DecodePatch(jsonPatch)
	if err != nil {
		panic(err)
	}

	modifiedJSON, err = patch.Apply(objectToPatchJSON)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(modifiedJSON, &modifiedObj)
	if err != nil {
		return err
	}

	f.ObjectStore.PutObject(modifiedObj, object_store.NewMetaIndex(kind, namespace, name))

	return nil
}

func (f *KubernetesPatch) DeleteObject(_, kind, namespace, name, _ string) error {
	f.ObjectStore.DeleteObject(object_store.NewMetaIndex(kind, namespace, name))
	return nil
}

func (f *KubernetesPatch) DeleteObjectInBackground(_, kind, namespace, name, _ string) error {
	f.ObjectStore.DeleteObject(object_store.NewMetaIndex(kind, namespace, name))
	return nil
}

func (f *KubernetesPatch) DeleteObjectNonCascading(_, kind, namespace, name, _ string) error {
	f.ObjectStore.DeleteObject(object_store.NewMetaIndex(kind, namespace, name))
	return nil
}

func (f *KubernetesPatch) Apply(kpBytes []byte) error {
	parsedSpecs, err := object_patch.ParseSpecs(kpBytes)
	if err != nil {
		return err
	}

	var applyErrors = &multierror.Error{}
	for _, spec := range parsedSpecs {
		var operationError error

		switch spec.Operation {
		case object_patch.Create:
			operationError = f.CreateObject(&unstructured.Unstructured{Object: spec.Object}, spec.Subresource)
		case object_patch.CreateOrUpdate:
			operationError = f.CreateOrUpdateObject(&unstructured.Unstructured{Object: spec.Object}, spec.Subresource)
		case object_patch.Delete:
			operationError = f.DeleteObject(spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name, spec.Subresource)
		case object_patch.DeleteInBackground:
			operationError = f.DeleteObjectInBackground(spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name, spec.Subresource)
		case object_patch.DeleteNonCascading:
			operationError = f.DeleteObjectNonCascading(spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name, spec.Subresource)
		case object_patch.JQPatch:
			operationError = f.JQPatchObject(spec.JQFilter, spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name, spec.Subresource)
		case object_patch.MergePatch:
			jsonMergePatch, err := json.Marshal(spec.MergePatch)
			if err != nil {
				applyErrors = multierror.Append(applyErrors, err)
				continue
			}

			operationError = f.MergePatchObject(jsonMergePatch, spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name, spec.Subresource)
		case object_patch.JSONPatch:
			jsonJSONPatch, err := json.Marshal(spec.JSONPatch)
			if err != nil {
				applyErrors = multierror.Append(applyErrors, err)
				continue
			}

			operationError = f.JSONPatchObject(jsonJSONPatch, spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name, spec.Subresource)
		}

		if operationError != nil {
			applyErrors = multierror.Append(applyErrors, operationError)
		}
	}

	return applyErrors.ErrorOrNil()
}
