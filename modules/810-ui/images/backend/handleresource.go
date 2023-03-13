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

func (dh *resourceHandler) HandleList(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// List
	list, err := dh.informer.Lister().List(labels.Everything())
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
func (dh *resourceHandler) HandleGet(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	name := params.ByName("name")
	// Single object
	obj, err := dh.informer.Lister().Get(name)
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

func (dh *resourceHandler) HandleCreate(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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
	createdObj, err := dh.ri.Create(r.Context(), &obj, metav1.CreateOptions{})
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

func (dh *resourceHandler) HandleUpdate(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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
	updatedObj, err := dh.ri.Update(r.Context(), &obj, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("error updating object: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"error updating object"}`))
		return
	}

	data, _ := json.Marshal(updatedObj)
	w.Write(data)
}

func (dh *resourceHandler) HandleDelete(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	// Delete
	name := params.ByName("name")
	err := dh.ri.Delete(r.Context(), name, metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("error deleting object: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"error deleting object"}`))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
