package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
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
	{
		// Nodes
		gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"}
		informer, err := factory.ForResource(gvr)
		if err != nil {
			return nil, err
		}
		h := newReadHandler(informer, gvr)
		// informer.Informer().AddEventHandler()

		namespaced := false
		pathPrefix := getPathPrefix(gvr, namespaced, "k8s")
		namedPathPrefix := pathPrefix + "/:name"

		router.GET(pathPrefix, h.HandleList)
		router.GET(namedPathPrefix, h.HandleGet)
	}

	// CRUD with cluster-scoped custom resources that are expected to be present
	for _, gvr := range []schema.GroupVersionResource{
		{Group: "deckhouse.io", Version: "v1alpha1", Resource: "deckhousereleases"},
		{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"},
		{Group: "deckhouse.io", Version: "v1alpha1", Resource: "moduleconfigs"},
	} {
		namespaced := false
		collectionPath := getPathPrefix(gvr, namespaced, "k8s")
		namedItemPath := collectionPath + "/:name"

		informer := dynFactory.ForResource(gvr)
		h := newDynamicHandler(informer, dynClient, gvr)
		// informer.Informer().AddEventHandler()

		router.GET(collectionPath, h.HandleList)
		router.GET(namedItemPath, h.HandleGet)
		router.POST(collectionPath, h.HandleCreate)
		router.PUT(namedItemPath, h.HandleUpdate)
		router.DELETE(namedItemPath, h.HandleDelete)
	}

	// CRUD with Cloud Providers, along with that adding the provider to discovery if it is present
	// TODO in cloud provider, add known instance classes, known router paths
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

		namespaced := false
		collectionPath := getPathPrefix(gvr, namespaced, "k8s")
		namedItemPath := collectionPath + "/:name"

		informer := dynFactory.ForResource(gvr)
		h := newDynamicHandler(informer, dynClient, gvr)
		// informer.Informer().AddEventHandler()

		router.GET(collectionPath, h.HandleList)
		router.GET(namedItemPath, h.HandleGet)
		router.POST(collectionPath, h.HandleCreate)
		router.PUT(collectionPath, h.HandleUpdate)
		router.DELETE(namedItemPath, h.HandleDelete)

		// addClusterCRUDHandlers(router, dynFactory, dynClient, gvr)
		discovery["cloudProvider"] = strings.TrimSuffix(gvr.Resource, "instanceclasses")
	}

	// Discovery endpoint
	router.GET("/discovery", handleDiscovery(clientset, discovery))

	var wrapper http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		// CORS
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if r.Method == "OPTIONS" {
			return
		}

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

// type handler struct{}
// func (h *handler) OnAdd(obj interface{})               {} // TODO: broadcast meaningful update to websocket
// func (h *handler) OnUpdate(oldObj, newObj interface{}) {} // TODO: broadcast meaningful update to websocket
// func (h *handler) OnDelete(obj interface{})            {} // TODO: broadcast meaningful update to websocket

func getConfig() *appConfig {
	flagSet := flag.NewFlagSet("dashboard", flag.ExitOnError)
	klog.InitFlags(flagSet)

	port := flagSet.String("port", "8999", "port to listen on")
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

/*
Там примерно такая логика:

	Клиент подключается.

	Клиент ожидает пинги { type: "ping" } . Если их не будет, он будет считать коннекшн stale и переконнекчиваться.

	Клиент делает запрос { command: "subscribe", identifier: "{\"channel\": \"MyChannel\"}"}

	Клиент ожидает ответ { type: "confirm_subscription", identifier: "{\"channel\": \"MyChannel\"}"}

	Клиент ожидает сообщения в канал { identifier: "{\"channel\": \"MyChannel\"}", message: "SOME JSON"}

	Клиент может слать в канал  { identifier: "{\"channel\": \"MyChannel\"}", command: "message", data: "SOME JSON"}

*/
