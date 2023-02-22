package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
)

type appConfig struct {
	listenPort   string
	resyncPeriod time.Duration
	kubeConfig   *rest.Config
}

func main() {
	appConfig := getConfig()

	// Init factory for informers for well-known types
	clientset, err := kubernetes.NewForConfig(appConfig.kubeConfig)
	if err != nil {
		klog.Fatal(fmt.Errorf("creating clientset: %v", err.Error()))
	}
	factory := informers.NewSharedInformerFactory(clientset, appConfig.resyncPeriod)
	defer factory.Shutdown()

	// Init factory for informers for custom types
	dynClient, err := dynamic.NewForConfig(appConfig.kubeConfig)
	if err != nil {
		klog.Fatal(fmt.Errorf("creating dynamic client: %v", err.Error()))
	}
	dynFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, appConfig.resyncPeriod)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler, err := initHandlers(ctx, httprouter.New(), clientset, factory, dynClient, dynFactory)
	if err != nil {
		klog.Fatal(fmt.Errorf("initializing handlers: %v", err.Error()))
	}

	// Start informers all at once
	factory.Start(ctx.Done()) // Start processing these informers.
	klog.Info("Started informers.")
	// Wait for cache sync
	klog.Info("Waiting for initial sync of informers.")
	synced := factory.WaitForCacheSync(ctx.Done())
	for v, ok := range synced {
		if !ok {
			klog.Fatalf("caches failed to sync: %v", v)
		}
	}

	// Start dynamic informers all at once
	dynFactory.Start(ctx.Done())
	klog.Info("Started dynamic informers.")
	// Wait for cache sync for dynamic informers
	klog.Info("Waiting for initial sync of dynamic informers.")
	dynSynced := dynFactory.WaitForCacheSync(ctx.Done())
	for v, ok := range dynSynced {
		if !ok {
			klog.Fatalf("dynamic caches failed to sync: %v", v)
		}
	}
	klog.Info("Listening :" + appConfig.listenPort)

	klog.Error(http.ListenAndServe(":"+appConfig.listenPort, handler))
}

func initHandlers(
	ctx context.Context,
	router *httprouter.Router,
	clientset *kubernetes.Clientset,
	factory informers.SharedInformerFactory,
	dynClient *dynamic.DynamicClient,
	dynFactory dynamicinformer.DynamicSharedInformerFactory,
) (http.HandlerFunc, error) {

	// Readonly with known resources
	AddClusterReadHandlers(router, factory, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"})

	// CRUD with cluster-scoped custom resources that are expected to be present
	for _, gvr := range []schema.GroupVersionResource{
		{Group: "deckhouse.io", Version: "v1alpha1", Resource: "deckhousereleases"},
		{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"},
		{Group: "deckhouse.io", Version: "v1alpha1", Resource: "moduleconfigs"},
	} {
		addClusterCRUDHandlers(router, dynFactory, dynClient, gvr)
	}

	// CRUD with Cloud Providers, along with that adding the provider to discovery if it is present
	discovery := map[string]string{
		"cloudProvider":     "none",
		"kubernetesVersion": "unknown",
	}
	for _, gvr := range []schema.GroupVersionResource{
		{Group: "deckhouse.io", Version: "v1", Resource: "awsinstanceclasses"},
		{Group: "deckhouse.io", Version: "v1", Resource: "azureinstanceclasses"},
		{Group: "deckhouse.io", Version: "v1", Resource: "gcpinstanceclasses"},
		{Group: "deckhouse.io", Version: "v1", Resource: "openstackinstanceclasses"},
		{Group: "deckhouse.io", Version: "v1", Resource: "vsphereinstanceclasses"},
		{Group: "deckhouse.io", Version: "v1", Resource: "yandexinstanceclasses"},
	} {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_, err := dynClient.Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			if apierrors.IsForbidden(err) {
				// 403 is expected if the CRD is not present
				klog.V(5).Infof("CRD %s is not available: %v", gvr.String(), err)
				continue
			}
			return nil, err
		}

		addClusterCRUDHandlers(router, dynFactory, dynClient, gvr)
		discovery["cloudProvider"] = strings.TrimSuffix(gvr.Resource, "instanceclasses")

	}

	// Discovery endpoint
	router.GET("/discovery", handleDiscovery(clientset, discovery))

	var wrapper http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		// CORS
		// w.Header().Set("Access-Control-Allow-Origin", "*")
		// w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		// w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		// if r.Method == "OPTIONS" {
		// 	return
		// }

		klog.V(5).Infof("Request: %s %s", r.Method, r.URL.Path)
		router.ServeHTTP(w, r)
		// TODO: Use echo/v4. To log response status, we need to wrap the response writer or
		// use non-standard library. It still will help to handle path parameters.
	}

	return wrapper, nil
}

func handleDiscovery(clientset *kubernetes.Clientset, discovery map[string]string) func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	lock := sync.Mutex{}
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		// The version of the Kubernetes API server can change, so we need to check it every time
		kubeVersion, err := clientset.ServerVersion()
		if err != nil {
			klog.Errorf("failed to get kube version: %v", err)
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(err.Error()))
			return
		}
		v := kubeVersion.String()

		if discovery["kubernetesVersion"] != v {
			lock.Lock()
			discovery["kubernetesVersion"] = v
			lock.Unlock()
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(discovery)
	}
}

func getPathPrefix(gvr schema.GroupVersionResource, isNamespaced bool) string {
	pathPrefix := "/" + gvr.Resource
	if len(gvr.Group) > 0 {
		pathPrefix = "/" + gvr.Group + pathPrefix
	}
	if isNamespaced {
		pathPrefix = "/namespaces/:namespace" + pathPrefix
	}
	return pathPrefix
}

func AddClusterReadHandlers(router *httprouter.Router, factory informers.SharedInformerFactory, gvr schema.GroupVersionResource) {
	AddReadHandlers(router, factory, gvr, false)
}

func addNamespacedReadHandlers(router *httprouter.Router, factory informers.SharedInformerFactory, gvr schema.GroupVersionResource) {
	AddReadHandlers(router, factory, gvr, true)
}

func AddReadHandlers(router *httprouter.Router, factory informers.SharedInformerFactory, gvr schema.GroupVersionResource, isNamespaced bool) {
	pathPrefix := getPathPrefix(gvr, isNamespaced)
	namedPathPrefix := pathPrefix + "/:name"

	informer, err := factory.ForResource(gvr)
	if err != nil {
		// The error only spawns on unrecognized GVRs synchronous switch/case statement
		klog.Fatal(fmt.Errorf("creating informer for %s: %v", gvr.String(), err.Error()))
	}

	// List
	router.GET(pathPrefix, func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		// List
		list, err := informer.Lister().List(labels.Everything())
		if err != nil {
			klog.Errorf("error listing %s: %v", gvr.Resource, err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("error listing %s", gvr.Resource),
			})
			return
		}

		data, _ := json.Marshal(list)
		w.Write(data)
	})

	// Item by name
	router.GET(namedPathPrefix, func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		name := params.ByName("name")
		obj, exists, err := informer.Informer().GetIndexer().GetByKey(name)
		if err != nil {
			klog.Errorf("error listing %s: %v", gvr.Resource, err)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("error getting %s", gvr.Resource),
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
	})
}

func addClusterCRUDHandlers(router *httprouter.Router, factory dynamicinformer.DynamicSharedInformerFactory, client *dynamic.DynamicClient, gvr schema.GroupVersionResource) {
	addCRUDHandlers(router, factory, client, gvr, false)
}

func addNamespacedCRUDHandlers(router *httprouter.Router, factory dynamicinformer.DynamicSharedInformerFactory, client *dynamic.DynamicClient, gvr schema.GroupVersionResource) {
	addCRUDHandlers(router, factory, client, gvr, true)
}

func addCRUDHandlers(router *httprouter.Router, factory dynamicinformer.DynamicSharedInformerFactory, client *dynamic.DynamicClient, gvr schema.GroupVersionResource, isNamespaced bool) {
	pathPrefix := getPathPrefix(gvr, isNamespaced)
	namedPathPrefix := pathPrefix + "/:name"

	informer := factory.ForResource(gvr)
	resourceInterface := client.Resource(gvr)

	// List
	router.GET(pathPrefix, func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		// List
		// TODO: accept label selectors
		list, err := informer.Lister().List(labels.Everything())
		if err != nil {
			klog.Errorf("error listing %s: %v", gvr.Resource, err)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("error listing %s", gvr.Resource),
			})
			return
		}

		data, _ := json.Marshal(list)
		w.Write(data)
	})

	// Item by name
	router.GET(namedPathPrefix, func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		name := params.ByName("name")
		// Single object
		obj, exists, err := informer.Informer().GetIndexer().GetByKey(name)
		if err != nil {
			klog.Errorf("error listing %s: %v", gvr.Resource, err)
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
	})

	// Creation
	router.POST(pathPrefix, func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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
		createdObj, err := resourceInterface.Create(r.Context(), &obj, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("error creating object: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"error creating object"}`))
			return
		}

		w.WriteHeader(http.StatusCreated)
		data, _ := json.Marshal(createdObj)
		w.Write(data)
	})

	// Update
	router.PUT(namedPathPrefix, func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
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
		updatedObj, err := resourceInterface.Update(r.Context(), &obj, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("error updating object: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"error updating object"}`))
			return
		}

		data, _ := json.Marshal(updatedObj)
		w.Write(data)
	})

	//  Deletion
	router.DELETE(namedPathPrefix, func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		name := params.ByName("name")
		// Delete
		err := resourceInterface.Delete(r.Context(), name, metav1.DeleteOptions{})
		if err != nil {
			klog.Errorf("error deleting object: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"error deleting object"}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// type handler struct{}
// func (h *handler) OnAdd(obj interface{})               {} // TODO: broadcast meaningful update to websocket
// func (h *handler) OnUpdate(oldObj, newObj interface{}) {} // TODO: broadcast meaningful update to websocket
// func (h *handler) OnDelete(obj interface{})            {} // TODO: broadcast meaningful update to websocket

func getConfig() *appConfig {
	flagSet := flag.NewFlagSet("dashboard", flag.ExitOnError)
	klog.InitFlags(flagSet)

	port := flagSet.String("port", "8080", "port to listen on")
	resyncPeriod := flagSet.Duration("resyncPeriod-period", 10*time.Minute, "informers resyncPeriod period")
	// create the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		// create local config
		if !errors.Is(err, rest.ErrNotInCluster) {
			// the only recognized error
			klog.Fatal(fmt.Errorf("getting kube client config: %v", err.Error()))
		}

		var kubeconfig *string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = flagSet.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flagSet.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}

		err := flagSet.Parse(os.Args[1:])
		if err != nil {
			klog.Fatal(fmt.Errorf("parsing flags: %v", err.Error()))
		}

		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			klog.Fatal(fmt.Errorf("building kube client config: %v", err.Error()))
		}

	} else {
		err := flagSet.Parse(os.Args[1:])
		if err != nil {
			klog.Fatal(fmt.Errorf("parsing flags: %v", err.Error()))
		}
	}

	return &appConfig{
		listenPort:   *port,
		resyncPeriod: *resyncPeriod,
		kubeConfig:   config,
	}
}

// type CRUD interface {
// 	List(context.Context, labels.Selector, fields.Selector) (*unstructured.UnstructuredList, error)
// 	Get(context.Context, string, metav1.GetOptions) (*unstructured.Unstructured, error)
// 	Create(context.Context, *unstructured.Unstructured, metav1.CreateOptions) (*unstructured.Unstructured, error)
// 	Update(context.Context, *unstructured.Unstructured, metav1.UpdateOptions) (*unstructured.Unstructured, error)
// 	Delete(context.Context, string, metav1.DeleteOptions) error
// }

// type GroupResourceConfig struct {
// 	GroupVersionResource schema.GroupVersionResource
// 	Namespace            string
// 	Informer             cache.SharedIndexInformer
// }

// type ListQuery struct {
// 	Namespace     string
// 	Name          string
// 	LabelSelector metav1.LabelSelector
// }

// type ItemQuery struct {
// 	Namespace string
// 	Name      string
// }
// type ApiGroupHandler interface {
// 	ApiGroup() string
// 	Resource() string
// 	Namespaced() bool

// 	ListHandler() func(ListQuery) (interface{}, error)
// 	ItemHandler() func(ItemQuery) (interface{}, error)
// 	UpdateHandler() func(obj interface{}) error
// 	CreateHandler() func(obj interface{}) error
// 	DeleteHandler() func(obj interface{}) error
// 	// Notify(Notifier)
// }

// func Register(mux http.ServeMux, gh ApiGroupHandler) {
// 	prefix := gh.ApiGroup() + "/" + gh.Resource()
// 	if gh.ListHandler() != nil {
// 		mux.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
// 			lq := ListQuery{}
// 			list, err := gh.ListHandler()(lq)
// 			if err != nil {
// 				w.WriteHeader(http.StatusInternalServerError)
// 			}
// 		})
// 	}
// 	mux.HandleFunc("/api/"+ApiGroupHandler.GroupVersion.Group+"/"+ApiGroupHandler.GroupVersion.Version+"/", ApiGroupHandler.Handle)
// }

// ii := theInformer.Informer()
// if err := ii.AddIndexers(cache.Indexers{
// 	"byName": func(obj interface{}) ([]string, error) {
// 		if node, ok := obj.(*unstructured.Unstructured); ok {
// 			name, found, err := unstructured.NestedString(node.Object, "metadata", "name")
// 			if err != nil {
// 				return nil, err
// 			}

// 			if !found {
// 				return nil, errors.New("name not found")
// 			}
// 		}
// 		return nil, nil
// 	},
// }); err != nil {
// 	klog.Fatal(fmt.Errorf("adding indexer: %v", err.Error()))
// }
