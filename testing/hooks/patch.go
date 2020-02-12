package hooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/deckhouse/deckhouse/testing/library/object_store"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	jsonpatch "gopkg.in/evanphx/json-patch.v4"
)

type KubernetesPatch struct {
	Operations []KubernetesPatchOperation
}

type KubernetesPatchOperation struct {
	Op           string `json:"op"`
	Namespace    string `json:"namespace,omitempty"`
	Resource     string `json:"resource,omitempty"`
	ResourceSpec string `json:"resourceSpec,omitempty"`
	JQFilter     string `json:"jqFilter,omitempty"`
	ApiVersion   string `json:"apiVersion,omitempty"`
	Kind         string `json:"kind,omitempty"`
	ResourceName string `json:"resourceName,omitempty"`
	NewStatus    string `json:"newStatus,omitempty"`
}

func NewKubernetesPatch(kpBytes []byte) (KubernetesPatch, error) {
	var kp KubernetesPatch

	lines := strings.Split(string(kpBytes), "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		operation := KubernetesPatchOperation{}
		err := json.Unmarshal([]byte(line), &operation)
		if err != nil {
			return KubernetesPatch{}, fmt.Errorf("failed to unmarshal JSON: %s\n\n%s", err, line)
		}
		kp.Operations = append(kp.Operations, operation)
	}
	return kp, nil
}

func (kp *KubernetesPatch) Apply(objectStore object_store.ObjectStore) (object_store.ObjectStore, error) {
	newObjectStore := objectStore
	var err error

	for _, kop := range kp.Operations { // []KubernetesPatchOperation
		newObjectStore, err = kop.Apply(newObjectStore)
		if err != nil {
			return object_store.ObjectStore{}, fmt.Errorf("failed to apply patch: %s", err)
		}
	}

	return newObjectStore, nil
}

func (kpo *KubernetesPatchOperation) Apply(objectStore object_store.ObjectStore) (object_store.ObjectStore, error) {
	newObjectStore := objectStore

	var err error
	switch kpo.Op {
	case "Create":
		var t interface{}
		dec := yaml.NewDecoder(strings.NewReader(kpo.ResourceSpec))
		err = dec.Decode(&t)
		if err != nil {
			return object_store.ObjectStore{}, fmt.Errorf("operation \"Create\", failed to decode YAML: %s\n", err)
		}
		if t == nil {
			return object_store.ObjectStore{}, errors.New("kubernetes Create operation should contain pure YAML")
		}

		var newObj unstructured.Unstructured
		newObj.SetUnstructuredContent(t.(map[string]interface{}))
		if newObjectStore.KubernetesResource(newObj.GetKind(), newObj.GetNamespace(), newObj.GetName()).Exists() {
			return object_store.ObjectStore{}, fmt.Errorf("kubernetes Create operation failed: resource %s/%s in ns %s is already exists", newObj.GetKind(), newObj.GetName(), newObj.GetNamespace())
		}
		newObjectStore.PutObject(newObj.Object, object_store.NewMetaIndex(newObj.GetKind(), newObj.GetNamespace(), newObj.GetName()))

	case "CreateIfNotExists":
		var t interface{}
		dec := yaml.NewDecoder(strings.NewReader(kpo.ResourceSpec))
		err = dec.Decode(&t)
		if err != nil {
			return object_store.ObjectStore{}, fmt.Errorf("operation \"CreateIfNotExists\", faield to decode YAML: %s\n\n%s", err)
		}
		if t == nil {
			return object_store.ObjectStore{}, errors.New("kubernetes CreateIfNotExists operation should contain pure YAML")
		}

		var newObj unstructured.Unstructured
		newObj.SetUnstructuredContent(t.(map[string]interface{}))
		if !newObjectStore.KubernetesResource(newObj.GetKind(), newObj.GetNamespace(), newObj.GetName()).Exists() {
			newObjectStore.PutObject(newObj.Object, object_store.NewMetaIndex(newObj.GetKind(), newObj.GetNamespace(), newObj.GetName()))
		}

	case "Replace":
		var t interface{}
		dec := yaml.NewDecoder(strings.NewReader(kpo.ResourceSpec))
		err = dec.Decode(&t)
		if err != nil {
			return object_store.ObjectStore{}, fmt.Errorf("operation \"Replace\", faield to decode YAML: %s\n\n%s", err)
		}
		if t == nil {
			return object_store.ObjectStore{}, errors.New("kubernetes Replace operation should contain pure YAML")
		}

		var newObj unstructured.Unstructured
		newObj.SetUnstructuredContent(t.(map[string]interface{}))
		newObjectStore.PutObject(newObj.Object, object_store.NewMetaIndex(newObj.GetKind(), newObj.GetNamespace(), newObj.GetName()))

	case "JQPatch":
		r := strings.Split(kpo.Resource, "/") // e.g. Ingress/mying
		kind, name := r[0], r[1]

		var objJSON []byte
		if obj, ok := newObjectStore.GetObject(object_store.NewMetaIndex(kind, kpo.Namespace, name)); ok {
			objJSON, err = json.Marshal(obj)
			if err != nil {
				return object_store.ObjectStore{}, fmt.Errorf("failed to marshal JSON object: %s\n\n%+v", err, obj)
			}
		} else {
			return object_store.ObjectStore{}, fmt.Errorf("can't find resource %s/%s/%s to patch", kind, kpo.Namespace, name)
		}

		stdin := bytes.Buffer{}
		stdout := bytes.Buffer{}
		stderr := bytes.Buffer{}
		cmd := exec.Command("jq", "-c", kpo.JQFilter)
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
		r := strings.Split(kpo.Resource, "/") // e.g. Ingress/mying
		kind, name := r[0], r[1]
		newObjectStore.DeleteObject(object_store.NewMetaIndex(kind, kpo.Namespace, name))

	case "StatusPatch":
		var objToPatch object_store.KubeObject
		var originalStatusJSON []byte
		var modifiedStatusJSON []byte
		var modifiedStatusObj interface{}

		if obj, ok := newObjectStore.GetObject(object_store.NewMetaIndex(kpo.Kind, kpo.Namespace, kpo.ResourceName)); ok {
			objToPatch = obj
			originalStatusJSON, err = json.Marshal(objToPatch["status"])
			if err != nil {
				return object_store.ObjectStore{}, fmt.Errorf("failed to marshal object's .status: %s\n\n%+v", err, objToPatch)
			}
		} else {
			return object_store.ObjectStore{}, fmt.Errorf("can't find resource %s/%s/%s to patch status", kpo.Kind, kpo.Namespace, kpo.ResourceName)
		}

		modifiedStatusJSON, err = jsonpatch.MergePatch(originalStatusJSON, []byte(kpo.NewStatus))
		if err != nil {
			return object_store.ObjectStore{}, err
		}

		err = json.Unmarshal(modifiedStatusJSON, &modifiedStatusObj)
		if err != nil {
			return object_store.ObjectStore{}, err
		}

		objToPatch["status"] = modifiedStatusObj
		newObjectStore.PutObject(objToPatch, object_store.NewMetaIndex(kpo.Kind, kpo.Namespace, kpo.ResourceName))

	case "StatusPut":
		var objToPatch object_store.KubeObject
		var modifiedStatusObj interface{}

		if obj, ok := newObjectStore.GetObject(object_store.NewMetaIndex(kpo.Kind, kpo.Namespace, kpo.ResourceName)); ok {
			objToPatch = obj
			if err != nil {
				return object_store.ObjectStore{}, fmt.Errorf("failed to marshal object's .status: %s\n\n%+v", err, objToPatch)
			}
		} else {
			return object_store.ObjectStore{}, fmt.Errorf("can't find resource %s/%s/%s to patch status", kpo.Kind, kpo.Namespace, kpo.ResourceName)
		}

		err = json.Unmarshal([]byte(kpo.NewStatus), &modifiedStatusObj)
		if err != nil {
			return object_store.ObjectStore{}, err
		}

		objToPatch["status"] = modifiedStatusObj
		newObjectStore.PutObject(objToPatch, object_store.NewMetaIndex(kpo.Kind, kpo.Namespace, kpo.ResourceName))
	}
	return newObjectStore, nil

}
