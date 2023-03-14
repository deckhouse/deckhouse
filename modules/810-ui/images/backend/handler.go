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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/klog/v2"
)

type resourceHandler struct {
	gvr      schema.GroupVersionResource
	informer informers.GenericInformer
	// ri is resourceInterface that is used namespaceable if `namespaced` is set to true
	ri         dynamic.NamespaceableResourceInterface
	namespaced bool
}

func newHandler(informer informers.GenericInformer, ri dynamic.NamespaceableResourceInterface, gvr schema.GroupVersionResource, namespaced bool) *resourceHandler {
	return &resourceHandler{gvr, informer, ri, namespaced}
}

func (h *resourceHandler) HandleList(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	q := r.URL.Query()
	labelSelector, err := labels.Parse(q.Get("labelSelector"))
	if err != nil {
		klog.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
	}

	var list []runtime.Object
	if h.namespaced {
		namespace := params.ByName("namespace")
		list, err = h.informer.Lister().ByNamespace(namespace).List(labelSelector)
	} else {
		list, err = h.informer.Lister().List(labelSelector)
	}
	if err != nil {
		err := fmt.Errorf("listing %s: %v", h.gvr.Resource, err)
		klog.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

// Item by name
func (h *resourceHandler) HandleGet(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var (
		obj runtime.Object
		err error
	)

	name := params.ByName("name")
	if h.namespaced {
		namespace := params.ByName("namespace")
		obj, err = h.informer.Lister().ByNamespace(namespace).Get(name)
	} else {
		obj, err = h.informer.Lister().Get(name)
	}

	if err != nil {
		if apierrors.IsNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		klog.Error(err)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(obj)
}

func (h *resourceHandler) HandleCreate(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		err := fmt.Errorf("reading body: %s", err)
		klog.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}
	var obj unstructured.Unstructured
	err = json.Unmarshal(body, &obj)
	if err != nil {
		err := fmt.Errorf("unmarshalling body: %s", err)
		klog.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	var createdObj *unstructured.Unstructured
	if h.namespaced {
		namespace := params.ByName("namespace")
		createdObj, err = h.ri.Namespace(namespace).Create(r.Context(), &obj, metav1.CreateOptions{})
	} else {
		createdObj, err = h.ri.Create(r.Context(), &obj, metav1.CreateOptions{})
	}
	if err != nil {
		err := fmt.Errorf("creating object: %s", err)
		klog.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(createdObj)
}

func (h *resourceHandler) HandleUpdate(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		err := fmt.Errorf("reading body: %s", err)
		klog.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}
	var obj unstructured.Unstructured
	err = json.Unmarshal(body, &obj)
	if err != nil {
		err := fmt.Errorf("unmarshalling body: %s", err)
		klog.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	var updatedObj *unstructured.Unstructured
	if h.namespaced {
		namespace := params.ByName("namespace")
		updatedObj, err = h.ri.Namespace(namespace).Update(r.Context(), &obj, metav1.UpdateOptions{})
	} else {
		updatedObj, err = h.ri.Update(r.Context(), &obj, metav1.UpdateOptions{})
	}
	if err != nil {
		err := fmt.Errorf("updating body: %s", err)
		klog.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updatedObj)
}

func (h *resourceHandler) HandleDelete(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	// Delete
	name := params.ByName("name")
	var err error
	if h.namespaced {
		namespace := params.ByName("namespace")
		err = h.ri.Namespace(namespace).Delete(r.Context(), name, metav1.DeleteOptions{})
	} else {
		err = h.ri.Delete(r.Context(), name, metav1.DeleteOptions{})
	}
	if err != nil {
		err := fmt.Errorf("deleting object: %s", err)
		klog.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
