package hooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/flant/shell-operator/pkg/kube/object_patch"
	jsonpatch "gopkg.in/evanphx/json-patch.v4"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

type KubernetesPatch []object_patch.OperationSpec

func NewKubernetesPatch(kpBytes []byte) (KubernetesPatch, error) {
	return object_patch.ParseSpecs(kpBytes)
}

func (kp *KubernetesPatch) Apply(objectStore object_store.ObjectStore) (object_store.ObjectStore, error) {
	newObjectStore := objectStore
	var err error

	for _, kop := range *kp {
		newObjectStore, err = applyPatch(kop, newObjectStore)
		if err != nil {
			return object_store.ObjectStore{}, fmt.Errorf("failed to apply patch: %s", err)
		}
	}

	return newObjectStore, nil
}

func applyPatch(kop object_patch.OperationSpec, objectStore object_store.ObjectStore) (object_store.ObjectStore, error) {
	newObjectStore := objectStore

	var err error
	switch kop.Operation {
	case "Create":
		var newObj unstructured.Unstructured
		newObj.SetUnstructuredContent(kop.Object)
		if newObjectStore.KubernetesResource(newObj.GetKind(), newObj.GetNamespace(), newObj.GetName()).Exists() {
			return object_store.ObjectStore{}, fmt.Errorf("kubernetes Create operation failed: resource %s/%s in ns %s is already exists", newObj.GetKind(), newObj.GetName(), newObj.GetNamespace())
		}
		newObjectStore.PutObject(newObj.Object, object_store.NewMetaIndex(newObj.GetKind(), newObj.GetNamespace(), newObj.GetName()))

	case "CreateOrUpdate":
		var newObj unstructured.Unstructured
		newObj.SetUnstructuredContent(kop.Object)
		newObjectStore.PutObject(newObj.Object, object_store.NewMetaIndex(newObj.GetKind(), newObj.GetNamespace(), newObj.GetName()))

	case "JQPatch":
		var objJSON []byte
		if obj, ok := newObjectStore.GetObject(object_store.NewMetaIndex(kop.Kind, kop.Namespace, kop.Name)); ok {
			objJSON, err = json.Marshal(obj)
			if err != nil {
				return object_store.ObjectStore{}, fmt.Errorf("failed to marshal JSON object: %s\n\n%+v", err, obj)
			}
		} else {
			return object_store.ObjectStore{}, fmt.Errorf("can't find resource %s/%s/%s to patch", kop.Kind, kop.Namespace, kop.Name)
		}

		stdin := bytes.Buffer{}
		stdout := bytes.Buffer{}
		stderr := bytes.Buffer{}
		cmd := exec.Command("jq", "-c", kop.JQFilter)
		cmd.Stdin = &stdin
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		stdin.Write(objJSON)

		err = cmd.Run()
		if err != nil {
			return object_store.ObjectStore{}, fmt.Errorf("failed to run jq: %s\n\n%+v", err, cmd)
		}
		patchedObjJSON := stdout.Bytes()

		var t interface{}
		err = json.Unmarshal(patchedObjJSON, &t)
		if err != nil {
			return object_store.ObjectStore{}, err
		}

		var patchedObj unstructured.Unstructured
		patchedObj.SetUnstructuredContent(t.(map[string]interface{}))
		newObjectStore.PutObject(patchedObj.Object, object_store.NewMetaIndex(patchedObj.GetKind(), patchedObj.GetNamespace(), patchedObj.GetName()))

	case "Delete":
		newObjectStore.DeleteObject(object_store.NewMetaIndex(kop.Kind, kop.Namespace, kop.Name))

	case "DeleteInBackground":
		newObjectStore.DeleteObject(object_store.NewMetaIndex(kop.Kind, kop.Namespace, kop.Name))

	case "MergePatch":
		var objToPatch object_store.KubeObject
		var originalStatusJSON []byte
		var modifiedStatusJSON []byte
		var modifiedStatusObj interface{}

		if obj, ok := newObjectStore.GetObject(object_store.NewMetaIndex(kop.Kind, kop.Namespace, kop.Name)); ok {
			objToPatch = obj
			originalStatusJSON, err = json.Marshal(objToPatch["status"])
			if err != nil {
				return object_store.ObjectStore{}, fmt.Errorf("failed to marshal object's .status: %s\n\n%+v", err, objToPatch)
			}
			if string(originalStatusJSON) == "null" {
				originalStatusJSON = []byte("{}")
			}
		} else {
			return object_store.ObjectStore{}, fmt.Errorf("can't find resource %s/%s/%s to patch", kop.Kind, kop.Namespace, kop.Name)
		}

		jsonPatchBytes, err := json.Marshal(kop.MergePatch["status"])
		if err != nil {
			return nil, err
		}

		modifiedStatusJSON, err = jsonpatch.MergePatch(originalStatusJSON, jsonPatchBytes)
		if err != nil {
			return object_store.ObjectStore{}, err
		}

		err = json.Unmarshal(modifiedStatusJSON, &modifiedStatusObj)
		if err != nil {
			return object_store.ObjectStore{}, err
		}

		objToPatch["status"] = modifiedStatusObj
		newObjectStore.PutObject(objToPatch, object_store.NewMetaIndex(kop.Kind, kop.Namespace, kop.Name))

	case "JSONPatch":
		var objToPatch object_store.KubeObject
		var statusJSON []byte
		var objectJSON []byte
		var modifiedJSON []byte
		var statusObj interface{}
		var modifiedObj object_store.KubeObject

		var patch jsonpatch.Patch

		if obj, ok := newObjectStore.GetObject(object_store.NewMetaIndex(kop.Kind, kop.Namespace, kop.Name)); ok {
			objToPatch = obj
			statusJSON, err = json.Marshal(objToPatch["status"])
			if err != nil {
				return object_store.ObjectStore{}, fmt.Errorf("failed to marshal object's .status: %s\n\n%+v", err, objToPatch)
			}
			if string(statusJSON) == "null" {
				statusJSON = []byte("{}")
			}

			err = json.Unmarshal(statusJSON, &statusObj)
			if err != nil {
				return object_store.ObjectStore{}, err
			}

			objToPatch["status"] = statusObj
		} else {
			return object_store.ObjectStore{}, fmt.Errorf("can't find resource %s/%s/%s to patch", kop.Kind, kop.Namespace, kop.Name)
		}

		objectJSON, err = json.Marshal(objToPatch)
		if err != nil {
			panic(err)
		}

		jsonPatchBytes, err := json.Marshal(kop.JSONPatch)
		if err != nil {
			return nil, err
		}

		patch, err = jsonpatch.DecodePatch(jsonPatchBytes)
		if err != nil {
			panic(err)
		}

		modifiedJSON, err = patch.Apply(objectJSON)
		if err != nil {
			panic(err)
		}

		err = json.Unmarshal(modifiedJSON, &modifiedObj)
		if err != nil {
			return object_store.ObjectStore{}, err
		}

		newObjectStore.PutObject(modifiedObj, object_store.NewMetaIndex(kop.Kind, kop.Namespace, kop.Name))
	}
	return newObjectStore, nil

}
