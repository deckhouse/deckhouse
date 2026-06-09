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

// nolint:gci
package controller

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	dvpapi "dvp-common/api"
)

const (
	providerConfigSecretName      = "d8-provider-cluster-configuration"
	providerConfigSecretNamespace = "kube-system"
	providerConfigDataKey         = "cloud-provider-cluster-configuration.yaml"

	nodeGroupLabel = "node.deckhouse.io/group"

	diskHostnameLabel    = "dvp.deckhouse.io/hostname"
	diskClusterUUIDLabel = "dvp.deckhouse.io/cluster-uuid"

	permanentNodeMigrationRequeue = 30 * time.Second
)

// permanentNodeProviderConfig holds the storageClass fields we care about.
// MasterNodeGroup and NodeGroups are typed as any in the shared type — define local structs here.
type permanentNodeProviderConfig struct {
	MasterNodeGroup *masterNodeGroupSCCfg  `json:"masterNodeGroup,omitempty"`
	NodeGroups      []staticNodeGroupSCCfg `json:"nodeGroups,omitempty"`
}

type masterNodeGroupSCCfg struct {
	InstanceClass masterInstanceClassSCCfg `json:"instanceClass"`
}

type masterInstanceClassSCCfg struct {
	RootDisk        *diskSCCfg  `json:"rootDisk,omitempty"`
	EtcdDisk        *diskSCCfg  `json:"etcdDisk,omitempty"`
	AdditionalDisks []diskSCCfg `json:"additionalDisks,omitempty"`
}

type staticNodeGroupSCCfg struct {
	Name          string                   `json:"name"`
	InstanceClass staticInstanceClassSCCfg `json:"instanceClass"`
}

type staticInstanceClassSCCfg struct {
	RootDisk        *diskSCCfg  `json:"rootDisk,omitempty"`
	AdditionalDisks []diskSCCfg `json:"additionalDisks,omitempty"`
}

type diskSCCfg struct {
	StorageClass string `json:"storageClass,omitempty"`
}

type nodeGroupSCConfig struct {
	RootDiskSC       string
	EtcdDiskSC       string // only meaningful for master node group
	AdditionalDiskSC []string
}

// PermanentNodeSCMigrationReconciler migrates VirtualDisk StorageClasses for
// terraform-managed (permanent) nodes when the provider cluster configuration changes.
// Worker nodes are handled separately by DeckhouseMachineReconciler via CAPI.
type PermanentNodeSCMigrationReconciler struct {
	Client      client.Client
	DVP         *dvpapi.DVPCloudAPI
	ClusterUUID string
}

func (r *PermanentNodeSCMigrationReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, req.NamespacedName, secret); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	cfg, err := parsePermanentNodeProviderConfig(secret.Data[providerConfigDataKey])
	if err != nil {
		log.Error(err, "failed to parse provider cluster configuration, skipping")
		return reconcile.Result{}, nil
	}

	anyMigrating := r.reconcileAllPermanentNodes(ctx, cfg)
	if anyMigrating {
		return reconcile.Result{RequeueAfter: permanentNodeMigrationRequeue}, nil
	}
	return reconcile.Result{}, nil
}

func (r *PermanentNodeSCMigrationReconciler) reconcileAllPermanentNodes(
	ctx context.Context,
	cfg *permanentNodeProviderConfig,
) bool {
	anyMigrating := false

	if cfg.MasterNodeGroup != nil {
		ic := cfg.MasterNodeGroup.InstanceClass
		ngCfg := nodeGroupSCConfig{
			RootDiskSC:       scString(ic.RootDisk),
			EtcdDiskSC:       scString(ic.EtcdDisk),
			AdditionalDiskSC: scSlice(ic.AdditionalDisks),
		}
		migrating, err := r.reconcileNodeGroup(ctx, "master", ngCfg)
		if err != nil {
			ctrl.LoggerFrom(ctx).Error(err, "failed to reconcile master node group")
		}
		anyMigrating = anyMigrating || migrating
	}

	for _, ng := range cfg.NodeGroups {
		ic := ng.InstanceClass
		ngCfg := nodeGroupSCConfig{
			RootDiskSC:       scString(ic.RootDisk),
			AdditionalDiskSC: scSlice(ic.AdditionalDisks),
		}
		migrating, err := r.reconcileNodeGroup(ctx, ng.Name, ngCfg)
		if err != nil {
			ctrl.LoggerFrom(ctx).Error(err, "failed to reconcile node group", "nodeGroup", ng.Name)
		}
		anyMigrating = anyMigrating || migrating
	}

	return anyMigrating
}

func (r *PermanentNodeSCMigrationReconciler) reconcileNodeGroup(
	ctx context.Context,
	nodeGroupName string,
	cfg nodeGroupSCConfig,
) (bool, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("nodeGroup", nodeGroupName)

	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList, client.MatchingLabels{nodeGroupLabel: nodeGroupName}); err != nil {
		return false, fmt.Errorf("list nodes for group %q: %w", nodeGroupName, err)
	}

	anyMigrating := false
	for _, node := range nodeList.Items {
		migrating, err := r.reconcileNode(ctx, node.Name, cfg)
		if err != nil {
			log.Error(err, "failed to reconcile node disks", "node", node.Name)
			continue
		}
		anyMigrating = anyMigrating || migrating
	}
	return anyMigrating, nil
}

func (r *PermanentNodeSCMigrationReconciler) reconcileNode(
	ctx context.Context,
	hostname string,
	cfg nodeGroupSCConfig,
) (bool, error) {
	disks, err := r.DVP.DiskService.ListDisksByLabels(ctx, map[string]string{
		diskHostnameLabel:    hostname,
		diskClusterUUIDLabel: r.ClusterUUID,
	})
	if err != nil {
		return false, fmt.Errorf("list disks for node %q: %w", hostname, err)
	}

	log := ctrl.LoggerFrom(ctx).WithValues("node", hostname)
	anyMigrating := false

	for i := range disks {
		vd := &disks[i]
		desiredSC := desiredSCForDisk(vd.Name, cfg)
		if desiredSC == "" {
			continue
		}

		specSC := virtualDiskSpecStorageClass(vd)
		statusSC := vd.Status.StorageClassName

		if specSC == desiredSC && statusSC == desiredSC {
			continue
		}

		if specSC == desiredSC && statusSC != desiredSC {
			log.Info("disk SC migration in progress", "disk", vd.Name, "specSC", specSC, "statusSC", statusSC)
			anyMigrating = true
			continue
		}

		log.Info("patching disk SC", "disk", vd.Name, "currentSC", specSC, "desiredSC", desiredSC)
		if err := r.DVP.DiskService.MigrateDiskStorageClass(ctx, vd.Name, desiredSC); err != nil {
			return false, fmt.Errorf("migrate disk %q to SC %q: %w", vd.Name, desiredSC, err)
		}
		anyMigrating = true
	}

	return anyMigrating, nil
}

// desiredSCForDisk identifies disk type from its name and returns the desired StorageClass.
// Disk name patterns (from terraform locals):
//   - root:            {prefix}-{nodeGroup}-{nodeIndex}-{hash}
//   - kubernetes-data: {prefix}-{nodeGroup}-kubernetes-data-{nodeIndex}-{hash}
//   - additional:      {prefix}-{nodeGroup}-additional-disk-{diskIndex}-{nodeIndex}-{hash}
func desiredSCForDisk(diskName string, cfg nodeGroupSCConfig) string {
	if strings.Contains(diskName, "-kubernetes-data-") {
		return cfg.EtcdDiskSC
	}
	if idx, ok := parseAdditionalDiskIndex(diskName); ok {
		if idx < len(cfg.AdditionalDiskSC) {
			return cfg.AdditionalDiskSC[idx]
		}
		return ""
	}
	return cfg.RootDiskSC
}

// parseAdditionalDiskIndex extracts the disk_index from a name like *-additional-disk-{N}-*.
func parseAdditionalDiskIndex(diskName string) (int, bool) {
	const marker = "-additional-disk-"
	idx := strings.Index(diskName, marker)
	if idx == -1 {
		return 0, false
	}
	rest := diskName[idx+len(marker):]
	parts := strings.SplitN(rest, "-", 2)
	n, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, false
	}
	return n, true
}

func parsePermanentNodeProviderConfig(raw []byte) (*permanentNodeProviderConfig, error) {
	if len(raw) == 0 {
		return &permanentNodeProviderConfig{}, nil
	}
	var cfg permanentNodeProviderConfig
	// sigs.k8s.io/yaml converts YAML → JSON then unmarshals using json tags
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal provider config: %w", err)
	}
	return &cfg, nil
}

func scString(d *diskSCCfg) string {
	if d == nil {
		return ""
	}
	return d.StorageClass
}

func scSlice(disks []diskSCCfg) []string {
	out := make([]string, len(disks))
	for i, d := range disks {
		out[i] = d.StorageClass
	}
	return out
}

func (r *PermanentNodeSCMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{},
			builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
				return obj.GetName() == providerConfigSecretName &&
					obj.GetNamespace() == providerConfigSecretNamespace
			})),
		).
		Named("permanentnode-sc-migration").
		Complete(r)
}
