package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/julienschmidt/httprouter"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/klog/v2"
)

type namespacedResourceHandler struct {
	gvr      schema.GroupVersionResource
	ri       dynamic.NamespaceableResourceInterface
	informer informers.GenericInformer
}

func newNamespacedHandler(informer informers.GenericInformer, ri dynamic.NamespaceableResourceInterface, gvr schema.GroupVersionResource) *namespacedResourceHandler {
	return &namespacedResourceHandler{gvr, ri, informer}
}

func (dh *namespacedResourceHandler) HandleList(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	namespace := params.ByName("namespace")

	list, err := dh.informer.Lister().ByNamespace(namespace).List(labels.Everything())
	if err != nil {
		klog.Errorf("error listing %s: %v", dh.gvr.Resource, err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("error listing %s", dh.gvr.Resource),
		})
		return
	}

	data, _ := json.Marshal(list)
	w.Write(data)
}

// Item by name
func (dh *namespacedResourceHandler) HandleGet(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	name := params.ByName("name")
	namespace := params.ByName("namespace")
	// Single object
	obj, err := dh.informer.Lister().ByNamespace(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
			return
		}
		klog.Errorf("error listing %s: %v", dh.gvr.Resource, err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"error getting item"}`))
		return
	}
	data, _ := json.Marshal(obj)
	w.Write(data)
}

func (dh *namespacedResourceHandler) HandleCreate(w http.ResponseWriter, r *http.Request, params httprouter.Params) {

	namespace := params.ByName("namespace")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		klog.Errorf("error reading body: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"error reading body"}`))
		return
	}
	var obj unstructured.Unstructured
	err = json.Unmarshal(body, &obj)
	if err != nil {
		klog.Errorf("error unmarshalling body: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"error unmarshalling body"}`))
		return
	}

	createdObj, err := dh.ri.Namespace(namespace).Create(r.Context(), &obj, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("error creating object: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"error creating object"}`))
		return
	}

	w.WriteHeader(http.StatusCreated)
	data, _ := json.Marshal(createdObj)
	w.Write(data)
}

func (dh *namespacedResourceHandler) HandleUpdate(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	namespace := params.ByName("namespace")

	// Update
	body, err := io.ReadAll(r.Body)
	if err != nil {
		klog.Errorf("error reading body: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"error reading body"}`))
		return
	}
	var obj unstructured.Unstructured
	err = json.Unmarshal(body, &obj)
	if err != nil {
		klog.Errorf("error unmarshalling body: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"error unmarshalling body"}`))
		return
	}
	updatedObj, err := dh.ri.Namespace(namespace).Update(r.Context(), &obj, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("error updating object: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"error updating object"}`))
		return
	}

	data, _ := json.Marshal(updatedObj)
	w.Write(data)
}

func (dh *namespacedResourceHandler) HandleDelete(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	// Delete
	name := params.ByName("name")
	namespace := params.ByName("namespace")

	err := dh.ri.Namespace(namespace).Delete(r.Context(), name, metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("error deleting object: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"error deleting object"}`))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
