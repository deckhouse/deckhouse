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

	"controller/internal/naming"
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

	// podSecurityProfileClusterDefault asks the template to render no pod-policy label at all, so
	// the namespace stays under the cluster default policy.
	podSecurityProfileClusterDefault = ""
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
// existing namespace on the defaults would drop an isolating NetworkPolicy into it and put the
// workloads that run there today under a Pod Security Standard the namespace does not have.
//
// The labels and annotations of the namespace itself are left out. The release renders only what
// the Project declares, and the three-way merge Helm applies it with never touches a key that no
// render mentioned, so a label such as istio.io/rev or a GitOps tracking key stays with whoever set
// it: nothing reverts a later change to it or brings it back once removed. The placement
// annotations are the one exception, see placementAnnotations.
//
// Only the keys the chosen template declares are emitted, so the result always validates against
// its parametersSchema.
func ParametersFor(namespace *corev1.Namespace, template string) map[string]any {
	params := make(map[string]any, 5)
	if annotations := placementAnnotations(namespace); len(annotations) > 0 {
		params["namespace"] = map[string]any{"annotations": annotations}
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

// podSecurityProfile maps the pod-policy label of a namespace to the parameter value that keeps the
// label as it is. The templates render a profile as its lower-case name, so only the three
// lower-case names map to a profile. Anything else, a missing label included, maps to the cluster
// default: the template then renders no label, and the release leaves the one the namespace has,
// if any, untouched. Mapping a missing label to a profile would move the namespace off the cluster
// default policy, and mapping any other value would rewrite it into a different one.
func podSecurityProfile(label string) string {
	switch label {
	case strings.ToLower(podSecurityProfileBaseline):
		return podSecurityProfileBaseline
	case strings.ToLower(podSecurityProfileRestricted):
		return podSecurityProfileRestricted
	case strings.ToLower(podSecurityProfilePrivileged):
		return podSecurityProfilePrivileged
	default:
		return podSecurityProfileClusterDefault
	}
}

// placementAnnotations returns the scheduler annotations of the namespace in the shape the
// namespace.annotations parameter takes. They are the only metadata of its own a namespace brings
// into its project, on purpose: the protective admission policy owns them on a project namespace,
// so nobody can change them there, and they are rendered from spec.nodeSelector and
// spec.tolerations of a template, which none of the templates adoption picks (simple, default,
// secure) declares. Left out, they would stay on the namespace in no one's desired state, with no
// way left to change or remove them; carried, they are changed through the Project like every other
// key the module owns.
func placementAnnotations(namespace *corev1.Namespace) map[string]any {
	out := make(map[string]any, 2)
	for _, key := range []string{naming.NodeSelectorAnnotation, naming.TolerationsAnnotation} {
		if value, ok := namespace.GetAnnotations()[key]; ok {
			out[key] = value
		}
	}
	return out
}
