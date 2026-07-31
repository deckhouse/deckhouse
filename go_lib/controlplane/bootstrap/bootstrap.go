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

// Package bootstrap performs the cluster-side part of the first control-plane bootstrap:
// RBAC bindings, control-plane marking of the node and upload of the d8-pki secret.
// It is a Go port of the bashible step
// candi/bashible/common-steps/cluster-bootstrap/072_install_control_plane.sh.tpl
// for immutable nodes, where no shell is available; the caller runs it right after the local
// kube-apiserver started answering /readyz.
//
// The client is expected to be built from super-admin.conf (group system:masters, bypasses RBAC):
// the bash step switches to admin.conf after creating the kubeadm:cluster-admins binding, here one
// super-admin client is enough because the "binding first, everything else after" order is kept.
package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/node"
)

var logger = log.Default().Named("controlplane-bootstrap")

const (
	// defaultNodeRegistrationTimeout and defaultNodeRegistrationInterval repeat the bash step:
	// 100 attempts with a 2s sleep in between.
	defaultNodeRegistrationTimeout  = 200 * time.Second
	defaultNodeRegistrationInterval = 2 * time.Second
)

type EnsureClusterObjectsOptions struct {
	// PKIDir holds the on-disk PKI uploaded into the d8-pki secret,
	// defaults to constants.DefaultCertificatesDir.
	PKIDir string
	// NodeRegistrationTimeout limits the wait for kubelet to register the Node, defaults to 200s.
	NodeRegistrationTimeout time.Duration
	// NodeRegistrationInterval is the Node poll interval, defaults to 2s.
	NodeRegistrationInterval time.Duration
}

// EnsureClusterObjects creates everything the freshly bootstrapped control plane needs in the
// cluster itself. Every operation is idempotent, so the caller may retry the whole function.
func EnsureClusterObjects(ctx context.Context, client kubernetes.Interface, nodeName string, opts EnsureClusterObjectsOptions) error {
	if nodeName == "" {
		return fmt.Errorf("ensure cluster objects: empty node name")
	}
	opts = optionsWithDefaults(opts)

	// kubeadm:cluster-admins goes first: admin.conf authenticates into that group and has no
	// permissions at all until the binding exists.
	if err := ensureClusterRoleBinding(ctx, client, clusterAdminsBinding()); err != nil {
		return err
	}

	if err := waitForNodeRegistration(ctx, client, nodeName, opts.NodeRegistrationTimeout, opts.NodeRegistrationInterval); err != nil {
		return err
	}

	if err := node.NewNodeManager(client).MarkAsControlPlane(nodeName); err != nil {
		return fmt.Errorf("mark node %s as control plane: %w", nodeName, err)
	}

	if err := ensurePKISecret(ctx, client, opts.PKIDir); err != nil {
		return err
	}

	return ensureClusterRoleBinding(ctx, client, apiserverKubeletClientBinding())
}

func optionsWithDefaults(opts EnsureClusterObjectsOptions) EnsureClusterObjectsOptions {
	if opts.PKIDir == "" {
		opts.PKIDir = constants.DefaultCertificatesDir
	}
	if opts.NodeRegistrationTimeout <= 0 {
		opts.NodeRegistrationTimeout = defaultNodeRegistrationTimeout
	}
	if opts.NodeRegistrationInterval <= 0 {
		opts.NodeRegistrationInterval = defaultNodeRegistrationInterval
	}

	return opts
}

// waitForNodeRegistration polls the Node until kubelet registers it. Any error is retried, not only
// NotFound: the local apiserver may still be rejecting requests while it finishes starting up.
func waitForNodeRegistration(ctx context.Context, client kubernetes.Interface, nodeName string, timeout, interval time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		_, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err == nil {
			logger.Info("node is registered", slog.String("node", nodeName))
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for node %s registration: %w (last error: %v)", nodeName, ctx.Err(), err)
		case <-time.After(interval):
		}
	}
}
