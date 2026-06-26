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

// Package render builds the per-namespace objects of a schema-based ProjectTemplate
// (deckhouse.io/v1alpha2) directly from its structured fields — the ADR-3 replacement for the
// Helm resourcesTemplate string. There is no template language in the path: each structured field
// maps to a concrete object (or a label/annotation), and a fromParam leaf is resolved against the
// project's effective parameters (the parametersSchema defaults merged with Project.spec.parameters).
//
// The rendered objects are intentionally bare: the heritage/project/project-template labels and the
// project namespace are injected downstream by the helm post-renderer, exactly as for the legacy
// resourcesTemplate path, so both paths share the same labelling, filtering and status bookkeeping.
package render

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/validate"
)

// Manifests renders the structured (v1alpha2) ProjectTemplate into a multi-document YAML of the
// per-namespace objects for the given project, resolving every fromParam leaf.
func Manifests(tmpl *v1alpha2.ProjectTemplate, project *v1alpha3.Project) (string, error) {
	params, err := effectiveParams(tmpl, project)
	if err != nil {
		return "", err
	}

	r := &renderer{name: project.Name, params: params}
	docs, err := r.build(&tmpl.Spec)
	if err != nil {
		return "", err
	}
	return marshalDocs(docs)
}

// effectiveParams overlays the template's parametersSchema defaults onto the project parameters,
// producing the values fromParam leaves resolve against. It mirrors the helm path's defaulting so a
// structured template and an equivalent resourcesTemplate see the same parameters.
func effectiveParams(tmpl *v1alpha2.ProjectTemplate, project *v1alpha3.Project) (map[string]any, error) {
	schema, err := validate.LoadSchema(tmpl.Spec.ParametersSchema.OpenAPIV3Schema)
	if err != nil {
		return nil, fmt.Errorf("load parameters schema of template %q: %w", tmpl.Name, err)
	}
	return validate.MergeDefaults(schema, project.Spec.Parameters), nil
}

type renderer struct {
	name   string
	params map[string]any
}

func (r *renderer) build(spec *v1alpha2.ProjectTemplateSpec) ([]map[string]any, error) {
	var docs []map[string]any

	ns, err := r.namespace(spec)
	if err != nil {
		return nil, err
	}
	docs = append(docs, ns)

	np, err := r.networkPolicy(spec)
	if err != nil {
		return nil, err
	}
	if np != nil {
		docs = append(docs, np)
	}

	plc, err := r.podLoggingConfig(spec)
	if err != nil {
		return nil, err
	}
	if plc != nil {
		docs = append(docs, plc)
	}

	docs = append(docs, r.operationPolicy())

	uids, hasUIDs, err := spec.AllowedUIDs.Resolve(r.params)
	if err != nil {
		return nil, fmt.Errorf("resolve allowedUIDs: %w", err)
	}
	gids, hasGIDs, err := spec.AllowedGIDs.Resolve(r.params)
	if err != nil {
		return nil, fmt.Errorf("resolve allowedGIDs: %w", err)
	}

	auditEnabled := false
	if spec.RuntimeAudit != nil {
		if v, ok, aErr := spec.RuntimeAudit.Enabled.Resolve(r.params); aErr != nil {
			return nil, fmt.Errorf("resolve runtimeAudit.enabled: %w", aErr)
		} else if ok {
			auditEnabled = v
		}
	}

	if auditEnabled && (hasUIDs || hasGIDs) {
		docs = append(docs, r.falcoAuditRules(uids, hasUIDs, gids, hasGIDs))
	}
	if hasUIDs || hasGIDs {
		docs = append(docs, r.securityPolicy(uids, hasUIDs, gids, hasGIDs))
	}

	return docs, nil
}

func (r *renderer) namespace(spec *v1alpha2.ProjectTemplateSpec) (map[string]any, error) {
	labels := map[string]any{}

	if psp, ok, err := spec.PodSecurityStandard.Resolve(r.params); err != nil {
		return nil, fmt.Errorf("resolve podSecurityStandard: %w", err)
	} else if ok && psp != "" {
		labels["security.deckhouse.io/pod-policy"] = strings.ToLower(psp)
	}

	if spec.Features != nil {
		if mon, ok, err := spec.Features.Monitoring.Resolve(r.params); err != nil {
			return nil, fmt.Errorf("resolve features.monitoring: %w", err)
		} else if ok && mon {
			labels["extended-monitoring.deckhouse.io/enabled"] = ""
		}
		if vs, ok, err := spec.Features.VulnerabilityScanning.Resolve(r.params); err != nil {
			return nil, fmt.Errorf("resolve features.vulnerabilityScanning: %w", err)
		} else if ok && vs {
			labels["security-scanning.deckhouse.io/enabled"] = ""
		}
	}

	annotations := map[string]any{}

	if tols, ok, err := spec.Tolerations.Resolve(r.params); err != nil {
		return nil, fmt.Errorf("resolve tolerations: %w", err)
	} else if ok && len(tols) > 0 {
		raw, mErr := json.Marshal(tols)
		if mErr != nil {
			return nil, fmt.Errorf("marshal tolerations: %w", mErr)
		}
		annotations["scheduler.alpha.kubernetes.io/defaultTolerations"] = string(raw)
	}

	if nodeSel, ok, err := spec.NodeSelector.Resolve(r.params); err != nil {
		return nil, fmt.Errorf("resolve nodeSelector: %w", err)
	} else if ok && len(nodeSel) > 0 {
		annotations["scheduler.alpha.kubernetes.io/node-selector"] = stringifyNodeSelector(nodeSel)
	}

	if spec.NamespaceMetadata != nil {
		if extra, ok, err := spec.NamespaceMetadata.Labels.Resolve(r.params); err != nil {
			return nil, fmt.Errorf("resolve namespaceMetadata.labels: %w", err)
		} else if ok {
			for k, v := range extra {
				labels[k] = v
			}
		}
		if extra, ok, err := spec.NamespaceMetadata.Annotations.Resolve(r.params); err != nil {
			return nil, fmt.Errorf("resolve namespaceMetadata.annotations: %w", err)
		} else if ok {
			for k, v := range extra {
				annotations[k] = v
			}
		}
	}

	metadata := map[string]any{"name": r.name}
	if len(labels) > 0 {
		metadata["labels"] = labels
	}
	if len(annotations) > 0 {
		metadata["annotations"] = annotations
	}

	return map[string]any{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata":   metadata,
	}, nil
}

func (r *renderer) networkPolicy(spec *v1alpha2.ProjectTemplateSpec) (map[string]any, error) {
	if spec.NetworkPolicy == nil {
		return nil, nil
	}
	mode, ok, err := spec.NetworkPolicy.Mode.Resolve(r.params)
	if err != nil {
		return nil, fmt.Errorf("resolve networkPolicy.mode: %w", err)
	}
	if !ok || mode != v1alpha2.NetworkPolicyModeIsolated {
		return nil, nil
	}

	nsSelector := func(name string) map[string]any {
		return map[string]any{"namespaceSelector": map[string]any{"matchLabels": map[string]any{"kubernetes.io/metadata.name": name}}}
	}
	withPod := func(nsName, podKey, podVal string) map[string]any {
		return map[string]any{
			"namespaceSelector": map[string]any{"matchLabels": map[string]any{"kubernetes.io/metadata.name": nsName}},
			"podSelector":       map[string]any{"matchLabels": map[string]any{podKey: podVal}},
		}
	}

	return map[string]any{
		"apiVersion": "networking.k8s.io/v1",
		"kind":       "NetworkPolicy",
		"metadata":   map[string]any{"name": "isolated"},
		"spec": map[string]any{
			"podSelector": map[string]any{"matchLabels": map[string]any{}},
			"policyTypes": []any{"Ingress", "Egress"},
			"ingress": []any{
				map[string]any{"from": []any{
					nsSelector(r.name),
					withPod("d8-monitoring", "app.kubernetes.io/name", "prometheus"),
					withPod("d8-ingress-nginx", "app", "controller"),
					withPod("d8-service-with-healthchecks", "app", "agent"),
				}},
			},
			"egress": []any{
				map[string]any{"to": []any{nsSelector(r.name)}},
				map[string]any{
					"to": []any{nsSelector("kube-system")},
					"ports": []any{
						map[string]any{"protocol": "UDP", "port": 53},
						map[string]any{"protocol": "TCP", "port": 53},
						map[string]any{"protocol": "UDP", "port": 5353},
						map[string]any{"protocol": "TCP", "port": 5353},
					},
				},
			},
		},
	}, nil
}

func (r *renderer) podLoggingConfig(spec *v1alpha2.ProjectTemplateSpec) (map[string]any, error) {
	if spec.LogShipping == nil {
		return nil, nil
	}
	ref, ok, err := spec.LogShipping.ClusterDestinationRef.Resolve(r.params)
	if err != nil {
		return nil, fmt.Errorf("resolve logShipping.clusterDestinationRef: %w", err)
	}
	if !ok || ref == "" {
		return nil, nil
	}
	return map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "PodLoggingConfig",
		"metadata":   map[string]any{"name": "default"},
		"spec":       map[string]any{"clusterDestinationRefs": []any{ref}},
	}, nil
}

func (r *renderer) operationPolicy() map[string]any {
	return map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "OperationPolicy",
		"metadata":   map[string]any{"name": "required-requests-" + shortHash(r.name)},
		"spec": map[string]any{
			"policies": map[string]any{"requiredResources": map[string]any{"requests": []any{"cpu", "memory"}}},
			"match":    r.nsMatch(),
		},
	}
}

func (r *renderer) securityPolicy(uids v1alpha2.IDRange, hasUIDs bool, gids v1alpha2.IDRange, hasGIDs bool) map[string]any {
	policies := map[string]any{}
	if hasGIDs {
		policies["runAsGroup"] = idRangePolicy(gids)
	}
	if hasUIDs {
		policies["runAsUser"] = idRangePolicy(uids)
	}
	return map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "SecurityPolicy",
		"metadata":   map[string]any{"name": "allowed-uid-gid-" + shortHash(r.name)},
		"spec": map[string]any{
			"enforcementAction": "Deny",
			"policies":          policies,
			"match":             r.nsMatch(),
		},
	}
}

func idRangePolicy(rng v1alpha2.IDRange) map[string]any {
	return map[string]any{
		"ranges": []any{map[string]any{"max": rng.Max, "min": rng.Min}},
		"rule":   "MustRunAs",
	}
}

func (r *renderer) falcoAuditRules(uids v1alpha2.IDRange, hasUIDs bool, gids v1alpha2.IDRange, hasGIDs bool) map[string]any {
	var cond strings.Builder
	cond.WriteString("spawned_process and container and proc.is_exe_upper_layer=true")
	if hasUIDs {
		fmt.Fprintf(&cond, " and user.uid > %d and user.uid < %d", uids.Min, uids.Max)
	}
	if hasGIDs {
		fmt.Fprintf(&cond, " and group.gid >= %d and group.gid <= %d", gids.Min, gids.Max)
	}
	fmt.Fprintf(&cond, " and k8s.ns.name=%s", r.name)

	const desc = "Detect if an executable not belonging to the base image of a container is being executed. " +
		"The drop and execute pattern can be observed very often after an attacker gained an initial foothold. " +
		"is_exe_upper_layer filter field only applies for container runtimes that use overlayfs as union mount filesystem."

	output := fmt.Sprintf("Executing binary not part of base image (project=%s user_loginuid=%%user.loginuid "+
		"user_uid=%%user.uid comm=%%proc.cmdline exe=%%proc.exe container_id=%%container.id k8s.ns=%%k8s.ns.name "+
		"image=%%container.image.repository proc.name=%%proc.name proc.sname=%%proc.sname proc.pname=%%proc.pname "+
		"proc.aname[2]=%%proc.aname[2] exe_flags=%%evt.arg.flags proc.exe_ino=%%proc.exe_ino "+
		"proc.exe_ino.ctime=%%proc.exe_ino.ctime proc.exe_ino.mtime=%%proc.exe_ino.mtime "+
		"proc.exe_ino.ctime_duration_proc_start=%%proc.exe_ino.ctime_duration_proc_start proc.exepath=%%proc.exepath "+
		"proc.cwd=%%proc.cwd proc.tty=%%proc.tty container.start_ts=%%container.start_ts proc.sid=%%proc.sid "+
		"proc.vpgid=%%proc.vpgid evt.res=%%evt.res)\n", r.name)

	return map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "FalcoAuditRules",
		"metadata":   map[string]any{"name": "container-drift-" + shortHash(r.name)},
		"spec": map[string]any{
			"rules": []any{
				map[string]any{"macro": map[string]any{"name": "spawned_process", "condition": "(evt.type in (execve, execveat) and evt.dir=<)"}},
				map[string]any{"macro": map[string]any{"name": "container", "condition": "(container.id != host)"}},
				map[string]any{"rule": map[string]any{
					"name":      fmt.Sprintf("Drop and execute new binary in container in %s project", r.name),
					"condition": cond.String(),
					"desc":      desc,
					"output":    output,
					"priority":  "Critical",
					"tags":      []any{"container_drift"},
				}},
			},
		},
	}
}

func (r *renderer) nsMatch() map[string]any {
	return map[string]any{
		"namespaceSelector": map[string]any{
			"labelSelector": map[string]any{
				"matchLabels": map[string]any{"kubernetes.io/metadata.name": r.name},
			},
		},
	}
}

// stringifyNodeSelector renders a node selector map into the scheduler annotation form "k1=v1,k2=v2".
// Keys are sorted so the annotation is deterministic across reconciles.
func stringifyNodeSelector(sel map[string]string) string {
	keys := make([]string, 0, len(sel))
	for k := range sel {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+sel[k])
	}
	return strings.Join(parts, ",")
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:8]
}

func marshalDocs(docs []map[string]any) (string, error) {
	var b strings.Builder
	for _, doc := range docs {
		raw, err := yaml.Marshal(doc)
		if err != nil {
			return "", fmt.Errorf("marshal %s/%s: %w", doc["apiVersion"], doc["kind"], err)
		}
		b.WriteString("---\n")
		b.Write(raw)
	}
	return b.String(), nil
}
