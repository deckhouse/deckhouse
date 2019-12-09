package object_store

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

type ErrObjectNotFound struct {
	message string
}

func (e *ErrObjectNotFound) Error() string {
	return e.message
}

func newErrObjectNotFound(index MetaIndex) *ErrObjectNotFound {
	return &ErrObjectNotFound{
		message: fmt.Sprintf("object by index %s/%s/%s not found in store", index.Kind, index.Namespace, index.Name),
	}
}

type KubeObject map[string]interface{}

func (obj KubeObject) Field(path string) gjson.Result {
	jsonBytes, _ := json.Marshal(obj)

	result := gjson.GetBytes(jsonBytes, path)

	return result
}

func (obj KubeObject) Exists() bool {
	return len(obj) > 0
}

type MetaIndex struct {
	Kind      string
	Namespace string
	Name      string
}

type ObjectStore map[MetaIndex]KubeObject

func (store ObjectStore) PutObject(object KubeObject, index MetaIndex) {
	store[lowerCaseMetaIndex(index)] = object
}

func (store ObjectStore) GetObject(index MetaIndex) (KubeObject, bool) {
	obj, ok := store[lowerCaseMetaIndex(index)]
	return obj, ok
}

func (store ObjectStore) DeleteObject(index MetaIndex) {
	delete(store, lowerCaseMetaIndex(index))
}

func (store ObjectStore) RetrieveObjectByMetaIndex(index MetaIndex) (object KubeObject, exists bool) {
	object, exists = store[index]

	return
}

func (store ObjectStore) KubernetesGlobalResource(kind, name string) KubeObject {
	metaIndex := lowerCaseMetaIndex(NewMetaIndex(kind, "", name))
	obj, _ := store.RetrieveObjectByMetaIndex(metaIndex)

	return obj
}

func (store ObjectStore) KubernetesResource(kind, namespace, name string) KubeObject {
	metaIndex := lowerCaseMetaIndex(NewMetaIndex(kind, namespace, name))
	obj, _ := store.RetrieveObjectByMetaIndex(metaIndex)

	return obj
}

func NewMetaIndex(kind, namespace, name string) MetaIndex {
	return MetaIndex{
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
	}
}

func lowerCaseMetaIndex(index MetaIndex) MetaIndex {
	return MetaIndex{
		Kind:      strings.ToLower(index.Kind),
		Namespace: strings.ToLower(index.Namespace),
		Name:      strings.ToLower(index.Name),
	}
}
