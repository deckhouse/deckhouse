package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/klog/v2"
)

type readHandler struct {
	gvr      schema.GroupVersionResource
	informer informers.GenericInformer
}

func newReadHandler(informer informers.GenericInformer, gvr schema.GroupVersionResource) *readHandler {
	return &readHandler{
		gvr,
		informer,
	}
}

// List
func (h *readHandler) HandleList(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// List
	list, err := h.informer.Lister().List(labels.Everything())
	if err != nil {
		klog.Errorf("error listing %s: %v", h.gvr.Resource, err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("error listing %s", h.gvr.Resource),
		})
		return
	}

	data, _ := json.Marshal(list)
	w.Write(data)
}

// Item by name
func (h *readHandler) HandleGet(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	name := params.ByName("name")
	obj, exists, err := h.informer.Informer().GetIndexer().GetByKey(name)
	if err != nil {
		klog.Errorf("error listing %s: %v", h.gvr.Resource, err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("error getting %s", h.gvr.Resource),
		})
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
