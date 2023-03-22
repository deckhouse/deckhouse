package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type resourceDefinition struct {
	gvr   schema.GroupVersionResource
	ns    bool         // is it namespaced?
	subh  []subHandler // subhandlers for a named object
	check gvrCheck     // should we register this API?
}

type gvrCheck func(context.Context, schema.GroupVersionResource) (bool, error)

type subHandler struct {
	method  string
	suffix  string
	handler func(*kubernetes.Clientset, *informerRegistry, schema.GroupVersionResource) httprouter.Handle
}

type informerRegistry struct {
	dynfactory dynamicinformer.DynamicSharedInformerFactory
	informers  map[string]informers.GenericInformer
}

func newInformerRegistry(dynFactory dynamicinformer.DynamicSharedInformerFactory) *informerRegistry {
	return &informerRegistry{
		dynfactory: dynFactory,
		informers:  make(map[string]informers.GenericInformer),
	}
}

func (r *informerRegistry) Add(gvr schema.GroupVersionResource) informers.GenericInformer {
	key := gvr.GroupResource().String()
	i := r.dynfactory.ForResource(gvr)
	r.informers[key] = i
	return i
}

func (r *informerRegistry) Get(gr schema.GroupResource) informers.GenericInformer {
	key := gr.String()
	if i, ok := r.informers[key]; ok {
		return i
	}
	return nil
}

func initHandlers(
	ctx context.Context,
	router *httprouter.Router,
	clientset *kubernetes.Clientset,
	factory informers.SharedInformerFactory, // FIXME seem to be unused
	dynClient *dynamic.DynamicClient,
	dynFactory dynamicinformer.DynamicSharedInformerFactory,
) (http.HandlerFunc, error) {
	reh := newResourceEventHandler()
	checkTimeout := 10 * time.Second

	definitions := []resourceDefinition{
		{
			gvr: schema.GroupVersionResource{Group: "", Resource: "nodes", Version: "v1"},
			subh: []subHandler{{
				method:  http.MethodPost,
				suffix:  "drain",
				handler: handleNodeDrain,
			}},
		},

		{
			gvr: schema.GroupVersionResource{Group: "", Resource: "secrets", Version: "v1"},
			ns:  true,
		},

		{
			gvr: schema.GroupVersionResource{Group: "apps", Resource: "deployments", Version: "v1"},
			ns:  true,
		},

		{
			gvr: schema.GroupVersionResource{Group: "deckhouse.io", Resource: "nodegroups", Version: "v1"},
			subh: []subHandler{{
				method:  http.MethodGet,
				suffix:  "scripts",
				handler: handleNodeGroupScripts,
			}},
		},
		{gvr: schema.GroupVersionResource{Group: "deckhouse.io", Resource: "deckhousereleases", Version: "v1alpha1"}},
		{gvr: schema.GroupVersionResource{Group: "deckhouse.io", Resource: "moduleconfigs", Version: "v1alpha1"}},
		{gvr: schema.GroupVersionResource{Group: "deckhouse.io", Resource: "dexauthenticators", Version: "v1alpha1"}},
		{gvr: schema.GroupVersionResource{Group: "deckhouse.io", Resource: "users", Version: "v1"}},

		{
			gvr:   schema.GroupVersionResource{Group: "deckhouse.io", Resource: "awsinstanceclasses", Version: "v1"},
			check: checkCustomResourceExistence(dynClient, checkTimeout),
		},
		{
			gvr:   schema.GroupVersionResource{Group: "deckhouse.io", Resource: "azureinstanceclasses", Version: "v1"},
			check: checkCustomResourceExistence(dynClient, checkTimeout),
		},
		{
			gvr:   schema.GroupVersionResource{Group: "deckhouse.io", Resource: "gcpinstanceclasses", Version: "v1"},
			check: checkCustomResourceExistence(dynClient, checkTimeout),
		},
		{
			gvr:   schema.GroupVersionResource{Group: "deckhouse.io", Resource: "openstackinstanceclasses", Version: "v1"},
			check: checkCustomResourceExistence(dynClient, checkTimeout),
		},
		{
			gvr:   schema.GroupVersionResource{Group: "deckhouse.io", Resource: "vsphereinstanceclasses", Version: "v1"},
			check: checkCustomResourceExistence(dynClient, checkTimeout),
		},
		{
			gvr:   schema.GroupVersionResource{Group: "deckhouse.io", Resource: "yandexinstanceclasses", Version: "v1"},
			check: checkCustomResourceExistence(dynClient, checkTimeout),
		},
	}

	// Adapter loop that both registers HTTP handlers and creates informers and subscription
	// handlers
	infReg := newInformerRegistry(dynFactory)
	discovery := newDiscoveryCollector(clientset)
	for _, def := range definitions {

		// Preliminary check for GVR that are expected to be absent
		gvr := def.gvr
		if def.check != nil {
			ok, err := def.check(ctx, gvr)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
		}

		// Router paths
		namespaced := def.ns
		collectionPath := getPathPrefix(gvr, namespaced, "k8s")
		namedItemPath := collectionPath + "/:name"

		// Using dynamic client for all resources since typed build-in informers do not
		// return kind and apiVersion from listers. Additionally, the code is unified.
		informer := infReg.Add(gvr)

		// Resource event handler will dispatch resource events to its subscribers
		_, _ = informer.Informer().AddEventHandler(reh.Handle(gvr.GroupResource()))

		// HTTP handlers. Despite we add handlers for all operations, some of them might be
		// unaccessible due to RBAC.
		h := newHandler(informer, dynClient.Resource(gvr), gvr, namespaced)
		router.GET(collectionPath, h.HandleList)
		router.GET(namedItemPath, h.HandleGet)
		router.POST(collectionPath, h.HandleCreate)
		router.PUT(namedItemPath, h.HandleUpdate)
		router.DELETE(namedItemPath, h.HandleDelete)

		// Additional HTTP handlers along with server paths discovery
		discovery.AddPath(collectionPath)
		discovery.AddPath(namedItemPath)
		for _, s := range def.subh {
			path := namedItemPath + "/" + s.suffix
			router.Handle(s.method, path, s.handler(clientset, infReg, gvr))
			discovery.AddPath(path)
		}

		// For cloud providers, there is a particular discovery means
		if strings.HasSuffix(gvr.Resource, "instanceclasses") {
			cloudProviderName := strings.TrimSuffix(gvr.Resource, "instanceclasses")
			discoveryCtx, discoveryCtxCancel := context.WithTimeout(ctx, checkTimeout)
			defer discoveryCtxCancel()
			if err := discovery.AddCloudProvider(discoveryCtx, cloudProviderName); err != nil {
				return nil, err
			}
		}
	}

	// Websocket
	sc := newSubscriptionController(reh, infReg)
	go sc.Start(ctx)
	router.GET("/subscribe", handleSubscribe(sc))

	// Discovery
	router.GET("/discovery", handleDiscovery(clientset, discovery.Build()))

	var wrapper http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		// CORS, should be opt-in by a flag for development purposes
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if r.Method == "OPTIONS" {
			return
		}

		// TODO: Use echo/v4. To log response status, we need to wrap the response writer or
		// use non-standard library. It still will help to handle path parameters.
		klog.V(5).Infof("Request: %s %s", r.Method, r.URL.Path)
		router.ServeHTTP(w, r)
	}

	return wrapper, nil
}

func checkCustomResourceExistence(dynClient *dynamic.DynamicClient, timeout time.Duration) func(context.Context, schema.GroupVersionResource) (bool, error) {
	return func(ctx context.Context, gvr schema.GroupVersionResource) (bool, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		_, err := dynClient.Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			if apierrors.IsForbidden(err) || apierrors.IsNotFound(err) {
				// 403 is expected if the CRD is not present locally, 404 is expected when run in a Pod
				klog.V(5).Infof("CRD %s is not available: %v", gvr.String(), err)
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
}

type resourceHandler struct {
	gvr      schema.GroupVersionResource
	informer informers.GenericInformer

	// ri is resourceInterface that is used as namespaceable if `namespaced` is set to true
	ri         dynamic.NamespaceableResourceInterface
	namespaced bool
}

func newHandler(informer informers.GenericInformer, ri dynamic.NamespaceableResourceInterface, gvr schema.GroupVersionResource, namespaced bool) *resourceHandler {
	return &resourceHandler{
		gvr:        gvr,
		informer:   informer,
		namespaced: namespaced,
		ri:         ri,
	}
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

func getPathPrefix(gvr schema.GroupVersionResource, isNamespaced bool, prefixes ...string) string {
	return "/" + strings.Join(getPathSegments(gvr, isNamespaced, prefixes...), "/")
}

func getPathSegments(gvr schema.GroupVersionResource, isNamespaced bool, prefixes ...string) []string {
	n := len(prefixes) + 1 // prefixes + resource
	if len(gvr.Group) > 0 {
		n++
	}
	if isNamespaced {
		n += 2
	}
	segments := make([]string, n)
	copy(segments, prefixes)
	i := len(prefixes)
	if len(gvr.Group) > 0 {
		segments[i] = gvr.Group
		i++
	}
	if isNamespaced {
		segments[i] = "namespaces"
		segments[i+1] = ":namespace"
		i += 2
	}
	segments[i] = gvr.Resource
	return segments
}
