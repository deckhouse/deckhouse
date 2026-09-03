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

package namespace

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// Built-in project templates the controller assigns to a namespace it adopts.
const (
	TemplateSimple  = "simple"
	TemplateDefault = "default"
	TemplateSecure  = "secure"
)

// Namespace labels the built-in templates render (see internal/render). They are read back both to
// pick a template for an existing namespace and to seed the project parameters with the state the
// namespace already has.
const (
	labelPodPolicy          = "security.deckhouse.io/pod-policy"
	labelExtendedMonitoring = "extended-monitoring.deckhouse.io/enabled"
	labelSecurityScanning   = "security-scanning.deckhouse.io/enabled"
)

// Pod Security Standard profiles, as the template parameter spells them.
const (
	podSecurityProfileBaseline   = "Baseline"
	podSecurityProfileRestricted = "Restricted"
	podSecurityProfilePrivileged = "Privileged"
)

// networkPolicyNotRestricted leaves the traffic open.
const networkPolicyNotRestricted = "NotRestricted"

// TemplateFor picks the built-in template matching what the namespace already carries, so adopting
// it does not change how the namespace behaves. A namespace with no template-rendered label gets
// the minimal template, which renders the namespace and nothing else.
func TemplateFor(namespace *corev1.Namespace) string {
	labels := namespace.GetLabels()
	if _, ok := labels[labelSecurityScanning]; ok {
		return TemplateSecure
	}
	if _, ok := labels[labelExtendedMonitoring]; ok {
		return TemplateDefault
	}
	if _, ok := labels[labelPodPolicy]; ok {
		return TemplateDefault
	}
	return TemplateSimple
}

// ParametersFor builds the project parameters that reproduce the current state of the namespace.
//
// The values are spelled out rather than left to the template defaults on purpose: the built-in
// templates default networkPolicy to Isolated and podSecurityProfile to Baseline, so adopting an
// existing namespace on the defaults would drop an isolating NetworkPolicy into it and tighten the
// Pod Security Standard on workloads that run there today.
//
// Only the keys the chosen template declares are emitted, so the result always validates against
// its parametersSchema.
func ParametersFor(namespace *corev1.Namespace, template string) map[string]any {
	params := make(map[string]any, 5)
	if meta := namespaceMeta(namespace); meta != nil {
		params["namespace"] = meta
	}

	if template == TemplateSimple {
		if len(params) == 0 {
			return nil
		}
		return params
	}

	labels := namespace.GetLabels()
	params["networkPolicy"] = networkPolicyNotRestricted
	params["podSecurityProfile"] = podSecurityProfile(labels[labelPodPolicy])
	_, monitoring := labels[labelExtendedMonitoring]
	params["extendedMonitoringEnabled"] = monitoring
	// default/secure always render a Deny OperationPolicy that requires CPU and
	// memory requests. An adopted namespace did not have that policy; turning it
	// on would stop existing workloads from rolling.
	params["requiredRequests"] = false

	if template == TemplateSecure {
		_, scanning := labels[labelSecurityScanning]
		params["securityScanningEnabled"] = scanning
	}

	return params
}

// podSecurityProfile maps the rendered pod-policy label back to the parameter value. A namespace
// without the label is reported as Privileged: the templates always render the label, so the only
// way to leave an adopted namespace as unrestricted as it is today is to ask for it explicitly. An
// unrecognised value is treated the same way, because guessing a stricter profile could evict
// running workloads.
func podSecurityProfile(label string) string {
	switch strings.ToLower(label) {
	case strings.ToLower(podSecurityProfileBaseline):
		return podSecurityProfileBaseline
	case strings.ToLower(podSecurityProfileRestricted):
		return podSecurityProfileRestricted
	default:
		return podSecurityProfilePrivileged
	}
}

// namespaceMeta mirrors the user-defined labels and annotations of the namespace into the shape the
// templates expect under the namespace parameter. It returns nil when there is nothing to mirror.
func namespaceMeta(namespace *corev1.Namespace) map[string]any {
	labels := filterUserMeta(namespace.GetLabels())
	annotations := filterUserMeta(namespace.GetAnnotations())

	meta := make(map[string]any, 2)
	if len(labels) > 0 {
		meta["labels"] = toAnyMap(labels)
	}
	if len(annotations) > 0 {
		meta["annotations"] = toAnyMap(annotations)
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

// managedMetaExact are platform-owned keys that must match in full. "heritage" is one of them:
// a HasPrefix match would also strip user keys such as heritageSomething.
var managedMetaExact = []string{
	"heritage",
	"app.kubernetes.io/managed-by",
	"kubernetes.io/metadata.name",
	labelPodPolicy,
	labelExtendedMonitoring,
	labelSecurityScanning,
}

// managedMetaPrefixes are platform-owned key prefixes that are never mirrored into project
// parameters. The controller applies them itself, and the three template-rendered labels are
// already represented by their own parameters.
var managedMetaPrefixes = []string{
	"projects.deckhouse.io/",
	"multitenancy.deckhouse.io/",
	"meta.helm.sh/",
	"kubectl.kubernetes.io/",
}

func filterUserMeta(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if isManagedMeta(k) {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func isManagedMeta(key string) bool {
	for _, exact := range managedMetaExact {
		if key == exact {
			return true
		}
	}
	for _, prefix := range managedMetaPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func toAnyMap(in map[string]string) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
