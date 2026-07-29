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

package virtualcontrolplaneconfiguration

import (
	"context"
	"fmt"
	"sort"
	"strings"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	konnectivityAgentCPContainerName = constants.VirtualKonnectivityAgentCPSecretName
	agentIdentifiersArgPrefix        = "--agent-identifiers="
	kubeApiserverManifestKey         = "kube-apiserver.yaml.tpl"
)

// parentEgressDestination is a parent-cluster address that the nested
// kube-apiserver must reach through konnectivity-agent-cp.
//
// ANP destHost only matches exact ipv4=/ipv6=/host= identifiers (cidr= is
// ignored), so each parent Service/endpoint IP that nested apiserver dials
// must be listed here. Tenant konnectivity-agents advertise default-route=true
// for tenant pod/Service traffic.
type parentEgressDestination struct {
	Name string
	IP   string
}

// reconcileKonnectivityAgentCPIdentifiers collects parent egress destinations
// and keeps konnectivity-agent-cp --agent-identifiers in sync on live
// StatefulSets and in the rendered kube-apiserver manifest (so ControlPlaneNode
// checksum/apply does not revert the live patch).
func (r *reconciler) reconcileKonnectivityAgentCPIdentifiers(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
) (reconcile.Result, error) {
	dests, res, err := r.collectParentEgressDestinations(ctx, vcp)
	if err != nil || !res.IsZero() {
		return res, err
	}
	if len(dests) == 0 {
		return reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
	}

	desiredArg := agentCPIdentifiersArg(dests)
	if desiredArg == agentIdentifiersArgPrefix {
		return reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
	}

	if err := r.ensureKonnectivityAgentCPIdentifiersOnStatefulSets(ctx, vcp, desiredArg); err != nil {
		return reconcile.Result{}, err
	}

	changed, err := r.ensureKonnectivityAgentCPIdentifiersInConfigSecret(ctx, vcp, desiredArg)
	if err != nil {
		return reconcile.Result{}, err
	}
	if changed {
		// Config checksum drives ControlPlaneNode → STS apply; requeue so CPN
		// picks up the updated manifest (this reconcile already passed CPN).
		return reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
	}

	return reconcile.Result{}, nil
}

// collectParentEgressDestinations loads parent Services the nested apiserver
// dials and returns their ClusterIPs for konnectivity-agent-cp destHost.
// Append new parent services here as they appear.
func (r *reconciler) collectParentEgressDestinations(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
) ([]parentEgressDestination, reconcile.Result, error) {
	var dests []parentEgressDestination

	// bashible aggregated API: nested Endpoints point at this parent ClusterIP.
	{
		ip, res, err := r.parentServiceClusterIP(ctx, vcp, bashibleServiceName)
		if err != nil || !res.IsZero() {
			return nil, res, err
		}
		dests = append(dests, parentEgressDestination{Name: bashibleServiceName, IP: ip})
	}

	return dests, reconcile.Result{}, nil
}

// parentServiceClusterIP returns ClusterIP of the per-VCP parent Service
// named VirtualResourceName(base, vcp.Name). Missing Service or empty
// ClusterIP → requeue (required destination).
func (r *reconciler) parentServiceClusterIP(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	baseName string,
) (string, reconcile.Result, error) {
	name := constants.VirtualResourceName(baseName, vcp.Name)
	svc, err := r.getService(ctx, vcp.Namespace, name)
	if apierrors.IsNotFound(err) {
		return "", reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
	}
	if err != nil {
		return "", reconcile.Result{}, fmt.Errorf("get parent Service %s/%s: %w", vcp.Namespace, name, err)
	}
	ip := serviceClusterIP(svc)
	if ip == "" {
		return "", reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
	}
	return ip, reconcile.Result{}, nil
}

func serviceClusterIP(svc *corev1.Service) string {
	if svc == nil {
		return ""
	}
	ip := svc.Spec.ClusterIP
	if ip == "" || ip == corev1.ClusterIPNone {
		return ""
	}
	return ip
}

// agentCPIdentifiersArg builds:
//
//	--agent-identifiers=ipv4=A&ipv4=B
//
// IPs are deduplicated and sorted for stable STS/Secret diffs.
func agentCPIdentifiersArg(dests []parentEgressDestination) string {
	seen := make(map[string]struct{}, len(dests))
	ips := make([]string, 0, len(dests))
	for _, d := range dests {
		if d.IP == "" {
			continue
		}
		if _, ok := seen[d.IP]; ok {
			continue
		}
		seen[d.IP] = struct{}{}
		ips = append(ips, d.IP)
	}
	sort.Strings(ips)

	parts := make([]string, 0, len(ips))
	for _, ip := range ips {
		parts = append(parts, "ipv4="+ip)
	}
	return agentIdentifiersArgPrefix + strings.Join(parts, "&")
}

func (r *reconciler) ensureKonnectivityAgentCPIdentifiersOnStatefulSets(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	desiredArg string,
) error {
	stsList := &appsv1.StatefulSetList{}
	if err := r.client.List(ctx, stsList,
		client.InNamespace(vcp.Namespace),
		client.MatchingLabels{
			"app": "kube-apiserver",
			constants.VirtualControlPlaneScopeLabelKey: vcp.Name,
		},
	); err != nil {
		return fmt.Errorf("list kube-apiserver StatefulSets: %w", err)
	}

	for i := range stsList.Items {
		sts := &stsList.Items[i]
		base := sts.DeepCopy()
		if !setKonnectivityAgentCPIdentifiersArg(&sts.Spec.Template.Spec, desiredArg) {
			continue
		}
		if err := r.patchStatefulSet(ctx, base, sts); err != nil {
			return fmt.Errorf("patch StatefulSet %s/%s konnectivity-agent-cp identifiers: %w",
				sts.Namespace, sts.Name, err)
		}
	}
	return nil
}

func (r *reconciler) ensureKonnectivityAgentCPIdentifiersInConfigSecret(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	desiredArg string,
) (bool, error) {
	name := constants.VirtualResourceName(constants.VirtualRenderedConfigSecretName, vcp.Name)
	secret, err := r.getSecret(ctx, vcp.Namespace, name)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get rendered config Secret: %w", err)
	}

	raw, ok := secret.Data[kubeApiserverManifestKey]
	if !ok {
		return false, fmt.Errorf("rendered config Secret missing %q", kubeApiserverManifestKey)
	}

	updated, changed := replaceAgentIdentifiersArgInManifest(string(raw), desiredArg)
	if !changed {
		return false, nil
	}

	base := secret.DeepCopy()
	secret.Data[kubeApiserverManifestKey] = []byte(updated)
	if err := r.patchSecret(ctx, base, secret); err != nil {
		return false, fmt.Errorf("patch rendered config Secret: %w", err)
	}
	return true, nil
}

func setKonnectivityAgentCPIdentifiersArg(podSpec *corev1.PodSpec, desiredArg string) bool {
	for i := range podSpec.Containers {
		c := &podSpec.Containers[i]
		if c.Name != konnectivityAgentCPContainerName {
			continue
		}
		return setAgentIdentifiersArg(&c.Args, desiredArg)
	}
	return false
}

func setAgentIdentifiersArg(args *[]string, desiredArg string) bool {
	for i, arg := range *args {
		if strings.HasPrefix(arg, agentIdentifiersArgPrefix) {
			if arg == desiredArg {
				return false
			}
			(*args)[i] = desiredArg
			return true
		}
	}
	*args = append(*args, desiredArg)
	return true
}

func replaceAgentIdentifiersArgInManifest(manifest, desiredArg string) (string, bool) {
	const marker = "name: " + konnectivityAgentCPContainerName

	idx := strings.Index(manifest, marker)
	if idx < 0 {
		return manifest, false
	}

	rest := manifest[idx:]
	argIdx := strings.Index(rest, agentIdentifiersArgPrefix)
	if argIdx < 0 {
		return manifest, false
	}

	abs := idx + argIdx
	lineStart := strings.LastIndex(manifest[:abs], "\n") + 1
	lineEndRel := strings.Index(manifest[abs:], "\n")
	lineEnd := len(manifest)
	if lineEndRel >= 0 {
		lineEnd = abs + lineEndRel
	}

	oldLine := manifest[lineStart:lineEnd]
	prefix := oldLine
	if i := strings.Index(oldLine, agentIdentifiersArgPrefix); i >= 0 {
		prefix = oldLine[:i]
	}
	newLine := prefix + desiredArg
	if oldLine == newLine {
		return manifest, false
	}

	return manifest[:lineStart] + newLine + manifest[lineEnd:], true
}

func (r *reconciler) patchStatefulSet(ctx context.Context, base, sts *appsv1.StatefulSet) error {
	return r.client.Patch(ctx, sts, client.MergeFrom(base))
}
