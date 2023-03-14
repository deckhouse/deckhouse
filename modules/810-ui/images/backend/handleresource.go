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

type resourceHandler struct {
	gvr      schema.GroupVersionResource
	ri       dynamic.ResourceInterface
	informer informers.GenericInformer
}

func newHandler(informer informers.GenericInformer, ri dynamic.ResourceInterface, gvr schema.GroupVersionResource) *resourceHandler {
	return &resourceHandler{gvr, ri, informer}
}

func (dh *resourceHandler) HandleList(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	// List
	q := r.URL.Query()
	labelSelector, err := labels.Parse(q.Get("labelSelector"))
	if err != nil {
		klog.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
	}
	list, err := dh.informer.Lister().List(labelSelector)
	if err != nil {
		err := fmt.Errorf("listing %s: %v", dh.gvr.Resource, err)
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
func (dh *resourceHandler) HandleGet(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	name := params.ByName("name")
	obj, err := dh.informer.Lister().Get(name)
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

func (dh *resourceHandler) HandleCreate(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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
	createdObj, err := dh.ri.Create(r.Context(), &obj, metav1.CreateOptions{})
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

func (dh *resourceHandler) HandleUpdate(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Update
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

	updatedObj, err := dh.ri.Update(r.Context(), &obj, metav1.UpdateOptions{})
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

func (dh *resourceHandler) HandleDelete(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	// Delete
	name := params.ByName("name")
	err := dh.ri.Delete(r.Context(), name, metav1.DeleteOptions{})
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
