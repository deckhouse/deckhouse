package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/drain"
)

// handleNodeDrain add synchronous drain operation. Draining cycle should be handled in Deckhouse
// and thus made explicit with use of annotations (if is has started, if a node drained). This
// handler should become deprecated in favor of the Deckhouse feature.
//
// TODO: remove when draining API is implemented in Deckhouse
func handleNodeDrain(clientset *kubernetes.Clientset, reg *informerRegistry, gvr schema.GroupVersionResource) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		name := params.ByName("name")

		informer := reg.Get(gvr.GroupResource())
		if informer == nil {
			panic(fmt.Sprintf("informer not registered for %v", gvr))
		}

		nodeGeneric, exists, err := informer.Informer().GetIndexer().GetByKey(name)
		if err != nil {
			klog.Errorf("error getting node %q: %v", name, err)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "error getting node"})
			return
		}
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
			return
		}

		node := nodeGeneric.(*v1.Node)

		var sb strings.Builder
		helper := &drain.Helper{
			Client:              clientset,
			Force:               true,
			IgnoreAllDaemonSets: true,
			DeleteEmptyDirData:  true,
			GracePeriodSeconds:  -1,
			// If a pod is not evicted in 5 minutes, delete the pod
			Timeout: 5 * time.Minute,
			Out:     io.Discard,
			ErrOut:  &sb,
			Ctx:     r.Context(),
		}
		if err := drain.RunCordonOrUncordon(helper, node, true); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			klog.ErrorS(err, "failed cordoning node", "name", name, "error", sb.String())
			_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("failed cordoning node: %v", err)})
			return
		}
		if err := drain.RunNodeDrain(helper, name); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			klog.ErrorS(err, "failed draining node", "name", name, "error", sb.String())
			_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("failed draining node: %v", err)})
			return
		}

		w.WriteHeader(http.StatusNoContent)
		w.Header().Set("Content-Type", "application/json")
	}
}

// handleNodeGroupScripts fetches bootstrap.sh and adopt.sh from corresponding NG secret.
func handleNodeGroupScripts(clientset *kubernetes.Clientset, reg *informerRegistry, ngGVR schema.GroupVersionResource) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		nodegroupName := params.ByName("name")

		// Verify setup
		ngInformer := reg.Get(ngGVR.GroupResource())
		if ngInformer == nil {
			panic(fmt.Sprintf("informer not registered for %v", ngGVR))
		}
		secretGR := schema.GroupResource{Resource: "secrets"}
		informer := reg.Get(secretGR)
		if informer == nil {
			panic(fmt.Sprintf("informer not registered for %v", secretGR))
		}

		// Ensure NodeGroup type is suitable to share scripts
		ngObj, err := ngInformer.Lister().Get(nodegroupName)
		if err != nil {
			klog.Errorf("error getting NodeGroup %q: %v", nodegroupName, err)
			if apierrors.IsNotFound(err) {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "error getting NodeGroup"})
			return
		}
		ng := ngObj.(*unstructured.Unstructured)
		ngType, ok, err := unstructured.NestedString(ng.UnstructuredContent(), "spec", "nodeType")
		if err != nil {
			klog.Errorf("reading spec.nodeType in NodeGroup %q: %v", nodegroupName, err)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "error getting NodeGroup"}) // generic error for client
			return
		}
		if !ok {
			klog.Errorf("error getting NodeGroup %q: %v", nodegroupName, err)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "error getting NodeGroup"}) // generic error for client
			return
		}
		// Check for allowed node types
		if ngType != "CloudStatic" && ngType != "Static" && ngType != "CloudPermanent" {
			// No scripts implied, though secrets might be there
			w.WriteHeader(http.StatusNotFound)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{})
			return
		}

		namespace := "d8-cloud-instance-manager"
		secretName := "manual-bootstrap-for-" + nodegroupName
		secretObj, err := informer.Lister().ByNamespace(namespace).Get(secretName)
		if err != nil {
			klog.Errorf("error getting secret %q: %v", secretName, err)
			if apierrors.IsNotFound(err) {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("error getting secret %s/%s", namespace, secretName)})
			return
		}

		var (
			secret     = secretObj.(*unstructured.Unstructured)
			data, _, _ = unstructured.NestedStringMap(secret.UnstructuredContent(), "data")

			adoptScript     = data["adopt.sh"]
			bootstrapScript = data["bootstrap.sh"]
			// TODO cleanupScripts = "bash /var/lib/bashible/node-cleanup.sh"
		)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"adopt.sh":     `base64 -d <<<'` + adoptScript + `' | bash`,
			"bootstrap.sh": `base64 -d <<<'` + bootstrapScript + `' | bash`,
		})
	}
}
