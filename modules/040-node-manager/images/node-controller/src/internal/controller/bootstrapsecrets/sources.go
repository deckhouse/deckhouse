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

package bootstrapsecrets

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/bootstrap"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/bashiblecontext"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
)

const (
	// The digests of every image the release ships, in the shape the bashible
	// templates read them: .images.registrypackages.<name>. bashible-apiserver
	// and the nodeconfig controller consume the same ConfigMap.
	imagesDigestsConfigMapName = "bashible-apiserver-files"
	imagesDigestsKey           = "images_digests.json"
)

// BuildInput collects everything the bootstrap templates read for one NodeGroup.
// The readers are bashiblecontext's, not copies: ReadEndpoints alone carries the
// pod-readiness rules a second implementation would drift from silently.
//
// token is what the render writes to /var/lib/bashible/bootstrap-token: a minted
// token for the Secrets this controller writes, the <<BOOTSTRAP_TOKEN>> literal
// for the MCM machine-class Secret, where machine-controller-manager substitutes
// a token of its own per machine.
func BuildInput(ctx context.Context, svc *bashiblecontext.Service, resolved derived_status.ResolvedNodeGroup, token string) (bootstrap.Input, error) {
	// Helm held this gate as `clusterUUID | required`: with an empty UUID rpp-get
	// asks the packages proxy for a prefix-less path, gets a 404, and the node
	// hangs for the whole bootstrap timeout instead of failing loudly.
	clusterUUID := svc.ReadGlobals(ctx).ClusterUUID
	if clusterUUID == "" {
		return bootstrap.Input{}, fmt.Errorf("build bootstrap input for NodeGroup %s: cluster UUID is empty", resolved.Name)
	}

	// The same gate for the same reason: ReadKubernetesCA returns "" on any failed
	// read, and an empty ca.crt yields a valid-looking Secret whose node cannot verify
	// the apiserver. The discover_kubernetes_ca hook returned that read error too.
	kubernetesCA := svc.ReadKubernetesCA()
	if kubernetesCA == "" {
		return bootstrap.Input{}, fmt.Errorf("build bootstrap input for NodeGroup %s: kubernetes CA is empty", resolved.Name)
	}

	endpoints, err := svc.ReadEndpoints(ctx)
	if err != nil {
		return bootstrap.Input{}, fmt.Errorf("read kube-apiserver endpoints: %w", err)
	}

	files, err := bootstrap.LoadFiles(ctx, svc.Client)
	if err != nil {
		return bootstrap.Input{}, err
	}

	images, err := readImages(ctx, svc.Client)
	if err != nil {
		return bootstrap.Input{}, err
	}

	cloudProvider := svc.ReadCloudProvider(ctx)
	provider, _ := cloudProvider["type"].(string)
	sshPublicKey, _ := cloudProvider["sshPublicKey"].(string)

	return bootstrap.Input{
		NodeGroup:              resolved.ToMap(),
		APIServerEndpoints:     endpoints.APIServerEndpoints,
		ClusterMasterEndpoints: endpoints.ClusterMasterEndpoints,
		ClusterUUID:            clusterUUID,
		Images:                 images,
		PackagesProxy:          map[string]any{"token": svc.ReadPackagesProxyToken(ctx)},
		// minget is 4KiB (crane export of the image candi/alt_base_images.yml
		// pins), so inlining its base64 into the script costs ~5KiB per copy.
		MingetB64:      base64.StdEncoding.EncodeToString(files.Binary("minget")),
		Provider:       provider,
		KubernetesCA:   kubernetesCA,
		BootstrapToken: token,
		SSHPublicKey:   sshPublicKey,
		Files:          files,
	}, nil
}

func readImages(ctx context.Context, r client.Reader) (map[string]any, error) {
	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: imagesDigestsConfigMapName}
	if err := r.Get(ctx, key, cm); err != nil {
		return nil, fmt.Errorf("read image digests %s: %w", key, err)
	}

	raw, ok := cm.Data[imagesDigestsKey]
	if !ok {
		return nil, fmt.Errorf("configmap %s has no %q key", key, imagesDigestsKey)
	}
	var images map[string]any
	if err := json.Unmarshal([]byte(raw), &images); err != nil {
		return nil, fmt.Errorf("parse %s of %s: %w", imagesDigestsKey, key, err)
	}
	return images, nil
}

// capiSecretNames returns every name a CAPI bootstrap Secret of this group must
// exist under: the per-zone name new MachineDeployments are given, plus the name
// each existing one already carries. The two differ on clusters migrated from the
// checksum-named Secret — a MachineDeployment keeps its dataSecretName forever,
// because changing a spec.template field replaces every node of the group.
func (r *Reconciler) capiSecretNames(ctx context.Context, ng *deckhousev1.NodeGroup, zones []string, clusterUUID string) ([]string, error) {
	names := make(map[string]struct{}, len(zones))
	for _, zone := range zones {
		// Mirrors the mdSuffix capi builds for a new MachineDeployment
		// (internal/controller/capi/capi_cloud.go:441).
		names[fmt.Sprintf("%s-%s", ng.Name, sha256Hash(clusterUUID+zone))] = struct{}{}
	}

	mds := &unstructured.UnstructuredList{}
	mds.SetGroupVersionKind(schema.GroupVersionKind{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "MachineDeploymentList"})
	if err := r.Client.List(ctx, mds,
		client.InNamespace(nodecommon.MachineNamespace),
		client.MatchingLabels{ngcommon.MachineDeploymentNodeGroupLabel: ng.Name},
	); err != nil {
		return nil, fmt.Errorf("list CAPI MachineDeployments of NodeGroup %s: %w", ng.Name, err)
	}
	for i := range mds.Items {
		md := &mds.Items[i]
		name, _, err := unstructured.NestedString(md.Object, "spec", "template", "spec", "bootstrap", "dataSecretName")
		if err != nil {
			return nil, fmt.Errorf("read dataSecretName of MachineDeployment %s: %w", md.GetName(), err)
		}
		if name != "" {
			names[name] = struct{}{}
		}
	}

	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	return out, nil
}

func sha256Hash(input string) string {
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", h)[:8]
}
