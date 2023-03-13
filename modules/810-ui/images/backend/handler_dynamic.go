package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/julienschmidt/httprouter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/klog/v2"
)

type dynamicHandler struct {
	gvr      schema.GroupVersionResource
	ri       dynamic.ResourceInterface
	informer informers.GenericInformer
}

func newHandler(informer informers.GenericInformer, ri dynamic.ResourceInterface, gvr schema.GroupVersionResource) *dynamicHandler {
	return &dynamicHandler{gvr, ri, informer}
}

func (dh *dynamicHandler) HandleList(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// List
	// TODO: accept label selectors
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
func (dh *dynamicHandler) HandleGet(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	name := params.ByName("name")
	// Single object
	obj, exists, err := dh.informer.Informer().GetIndexer().GetByKey(name)
	if err != nil {
		klog.Errorf("error listing %s: %v", dh.gvr.Resource, err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"error getting item"}`))
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
		return
	}
	data, _ := json.Marshal(obj)
	w.Write(data)
}

func (dh *dynamicHandler) HandleCreate(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

func (dh *dynamicHandler) HandleUpdate(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

func (dh *dynamicHandler) HandleDelete(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
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
