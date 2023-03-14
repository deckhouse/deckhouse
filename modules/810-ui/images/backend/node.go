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
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/drain"
)

// Draining cycle should be handled in Deckhouse and thus made excplicit with use of annotations (if
// is has started, if a node drained). This handler should become deprecated in favor of the
// Deckhouse feature.
func handleNodeDrain(clientset *kubernetes.Clientset, informer informers.GenericInformer) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		name := params.ByName("name")
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
