/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package noderename removes the Node object of a node that is about to come
// back under a different name.
//
// A Node object cannot be renamed, so /var/lib/bashible/rename_node.sh renames a
// node by having it register again. The old object has to be gone before the
// machine returns: while both exist they are one address wearing two names, and
// a CNI that keys its peers by address tears down the entry for one when the
// other goes away. A node may read the Node objects of its cluster but not
// delete them, so it asks here instead, by annotating itself.
//
// What makes this safe is that the request can only ever be about the node
// carrying it: NodeRestriction lets a kubelet write its own Node object and no
// other. The controller adds two conditions of its own - the node has to be one
// whose name is its own business, and it has to have stopped reporting, which is
// what rename_node.sh arranges by stopping kubelet before it asks.
package noderename

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("node-rename", &corev1.Node{}, &Reconciler{})
}

const (
	// RenameToAnnotation is how a node asks for its own Node object to be
	// removed so it can register again under the name the annotation carries.
	// rename_node.sh sets it, with kubelet already stopped.
	RenameToAnnotation = "node.deckhouse.io/rename-to"

	nodeTypeStatic = "Static"

	// How long to wait before looking again at a node that has asked but is
	// still reporting. Node events do not fire when a node merely goes quiet:
	// the Ready condition turns on a timer the API server keeps, so this is what
	// notices.
	retryInterval = 20 * time.Second
)

// nodeNameRe is the RFC 1123 DNS subdomain the API server will accept as an
// object name. A request for anything else could never be carried out.
var nodeNameRe = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(_ register.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("node", req.Name)

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	newName := node.Annotations[RenameToAnnotation]
	if newName == "" {
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("renameTo", newName)

	if reason := r.refuse(node, newName); reason != "" {
		logger.Info("refusing the rename request", "reason", reason)
		r.Recorder.Eventf(node, corev1.EventTypeWarning, "NodeRenameRejected",
			"Node %s asked to be renamed to %q, which was refused: %s", node.Name, newName, reason)
		return ctrl.Result{}, r.clearAnnotation(ctx, node)
	}

	// Only a node that has stopped reporting. rename_node.sh stops kubelet
	// before it asks, so a node still calling in is one whose request did not
	// come from the rename - or one whose kubelet has not gone quiet yet, which
	// takes as long as the API server's grace period.
	if isReady(node) {
		logger.V(1).Info("the node is still reporting; waiting for it to go quiet before removing its Node object")
		return ctrl.Result{RequeueAfter: retryInterval}, nil
	}

	logger.Info("removing the Node object so the machine can register under its new name")
	if err := r.Client.Delete(ctx, node); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("delete node %s: %w", node.Name, err)
	}

	r.Recorder.Eventf(node, corev1.EventTypeNormal, "NodeRenameAccepted",
		"Node object %s removed; the machine will register as %s", node.Name, newName)

	return ctrl.Result{}, nil
}

// refuse returns why the request cannot be carried out, or "" if it can.
func (r *Reconciler) refuse(node *corev1.Node, newName string) string {
	if nodeType := node.Labels[nodecommon.NodeTypeLabel]; nodeType != nodeTypeStatic {
		// Every other kind of node is found through its name by something in the
		// cloud: machine-controller-manager matches a Machine to its Node that way,
		// and a CloudStatic node - which is given no providerID at all - is how the
		// cloud controller manager finds the machine it has to initialize. Renamed,
		// such a node is matched by nothing.
		return fmt.Sprintf("a %s node is found by the cloud through the node's name", nodeType)
	}

	if newName == node.Name {
		return "the node already has that name"
	}

	if len(newName) > 253 || !nodeNameRe.MatchString(newName) {
		return "the name is not a valid RFC 1123 DNS subdomain"
	}

	return ""
}

// clearAnnotation takes a refused request off the node, so that a node whose
// request cannot be carried out is not asked about on every reconcile for the
// rest of its life.
func (r *Reconciler) clearAnnotation(ctx context.Context, node *corev1.Node) error {
	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				RenameToAnnotation: nil,
			},
		},
	})
	if err != nil {
		return err
	}

	if err := r.Client.Patch(ctx, node, client.RawPatch(types.MergePatchType, patch)); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("clear the %s annotation on node %s: %w", RenameToAnnotation, node.Name, err)
	}
	return nil
}

func isReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}
