# =============================================================================
# Library: lib.common
# =============================================================================
# Container iteration, field access helpers, and exception label utilities.
# =============================================================================
package lib.common

# Supported kinds for pod-spec extraction.
#
# This list is deliberately wider than the kinds the module's own generated
# constraints match: the `workload_kinds` Helm helper excludes ReplicaSet,
# because a ReplicaSet is created by a Deployment and its denial surfaces in
# the Deployment status instead. The library still resolves ReplicaSet so that
# a hand-written Constraint matching it behaves correctly.
workload_kind(kind) if {
  kind == "Pod"
}

workload_kind(kind) if {
  kind == "Deployment"
}

workload_kind(kind) if {
  kind == "StatefulSet"
}

workload_kind(kind) if {
  kind == "DaemonSet"
}

workload_kind(kind) if {
  kind == "ReplicaSet"
}

workload_kind(kind) if {
  kind == "ReplicationController"
}

workload_kind(kind) if {
  kind == "Job"
}

workload_kind(kind) if {
  kind == "CronJob"
}

# Normalize PodSpec location across pod-creating workloads.
# - Pod: spec
# - Deployment/StatefulSet/DaemonSet/ReplicaSet/ReplicationController/Job: spec.template.spec
# - CronJob: spec.jobTemplate.spec.template.spec
pod_spec := pod_spec_of(object.get(input.review, "object", {}))

# Pod spec for a given object: parameterized counterpart of pod_spec, for
# library functions that receive obj as a parameter.
pod_spec_of(obj) := out if {
  kind := object.get(obj, "kind", "")
  workload_kind(kind)
  out := pod_spec_for_kind(obj, kind)
}

# For unknown/absent kind, fall back to object.spec instead of {}.
# This prevents fail-open: if pod_spec is {}, input_containers is empty
# and every container-level check silently passes.
# For security modules, an unrecognised input should not mean "allowed".
pod_spec_of(obj) := out if {
  kind := object.get(obj, "kind", "")
  not workload_kind(kind)
  out := object.get(obj, "spec", {})
}

# Object normalized to a pod-like shape: its own metadata plus the resolved
# pod spec. Lets a spec-relative path such as ["spec", "hostNetwork"] resolve
# correctly for both Pods and controllers.
normalized_pod_object(obj) := {
  "metadata": object.get(obj, "metadata", {}),
  "spec": pod_spec_of(obj),
}

pod_spec_for_kind(obj, "Pod") := out if {
  out := object.get(obj, "spec", {})
}

pod_spec_for_kind(obj, "CronJob") := out if {
  out := object.get(obj, ["spec", "jobTemplate", "spec", "template", "spec"], {})
}

pod_spec_for_kind(obj, kind) := out if {
  kind != "Pod"
  kind != "CronJob"
  out := object.get(obj, ["spec", "template", "spec"], {})
}

# Backwards-compatible container iterator (uses input.review)
input_containers contains c if {
  c := pod_spec.containers[_]
}

input_containers contains c if {
  c := pod_spec.initContainers[_]
}

input_containers contains c if {
  c := pod_spec.ephemeralContainers[_]
}

# Parameterized container iterator (expects a pod spec)
input_containers_from(spec) := containers if {
  base := object.get(spec, "containers", [])
  init := object.get(spec, "initContainers", [])
  eph := object.get(spec, "ephemeralContainers", [])
  containers := array.concat(array.concat(base, init), eph)
}

has_field(object, field) if {
  object[field]
}

has_path(obj, path) if {
  object.get(obj, path, {"__missing__": true}) != {"__missing__": true}
}

get_field(obj, path, _default) := out if {
  out := object.get(obj, path, _default)
}

# Backwards-compatible exception label lookup (uses input.review)
get_exception_label(container) := label if {
  labels := object_labels
  key := sprintf("security.deckhouse.io/security-policy-exception.container.%v", [container.name])
  label := labels[key]
  label != ""
} else := label if {
  labels := object_labels
  key := sprintf("security.deckhouse.io/security-policy-exception.container.%v", [container.name])
  object.get(labels, key, "") == ""
  label := object.get(labels, "security.deckhouse.io/security-policy-exception", "")
  label != ""
} else := "" if {
  true
}

# Parameterized exception label lookup (uses labels map)
get_exception_label_from_labels(container, labels) := label if {
  key := sprintf("security.deckhouse.io/security-policy-exception.container.%v", [container.name])
  label := object.get(labels, key, "")
  label != ""
} else := label if {
  key := sprintf("security.deckhouse.io/security-policy-exception.container.%v", [container.name])
  object.get(labels, key, "") == ""
  label := object.get(labels, "security.deckhouse.io/security-policy-exception", "")
  label != ""
} else := "" if {
  true
}

# =============================================================================
# SPE label resolution helpers for controllers
# =============================================================================
# When a controller (Deployment, StatefulSet, etc.) is intercepted, the SPE
# label is read from the pod template's metadata.labels only
# (spec.template.metadata.labels, or spec.jobTemplate.spec.template.metadata.labels
# for CronJob). The controller's own top-level metadata.labels are never read
# and never merged in: those labels do not propagate to the pods the controller
# creates, so honouring them would let the controller pass while the pods it
# creates are still denied.
# =============================================================================

# Pod template metadata labels for the current review object.
# Derived from pod_template_metadata_for_kind so the Pod/CronJob/controller
# dispatch lives in exactly one place.
pod_template_metadata_labels_for_kind(obj, kind) := labels if {
  labels := object.get(pod_template_metadata_for_kind(obj, kind), "labels", {})
}

# Effective labels for SPE resolution from input.review.object.
# Thin wrapper over effective_labels: same Pod/controller/unknown-kind
# dispatch, sourced from input.review.
object_labels := effective_labels(object.get(input.review, "object", {}))

# Effective namespace for SPE resolution from input.review.object.
object_namespace := ns if {
  obj := object.get(input.review, "object", {})
  ns := object.get(obj, ["metadata", "namespace"], "")
}

# Effective labels for SPE resolution from a given object.
# Used by library functions that receive obj as a parameter.
# For Pods: uses the pod's own metadata.labels.
# For controllers: uses ONLY the pod template's metadata.labels.
effective_labels(obj) := labels if {
  kind := object.get(obj, "kind", "")
  kind == "Pod"
  labels := object.get(obj, ["metadata", "labels"], {})
}

effective_labels(obj) := labels if {
  kind := object.get(obj, "kind", "")
  kind != "Pod"
  workload_kind(kind)
  labels := pod_template_metadata_labels_for_kind(obj, kind)
}

# Fallback for unknown kinds: use the object's own metadata.labels, so SPE
# labels are not silently dropped for unrecognised objects. Mirrors the
# pod_spec fallback to object.spec for unknown kinds.
effective_labels(obj) := labels if {
  kind := object.get(obj, "kind", "")
  kind != "Pod"
  not workload_kind(kind)
  labels := object.get(obj, ["metadata", "labels"], {})
}

# Pod template metadata, the single dispatch point for both pod-template
# metadata and pod-template labels.
# For Pod: empty (no pod template)
# For CronJob: spec.jobTemplate.spec.template.metadata
# For other controllers: spec.template.metadata
pod_template_metadata_for_kind(obj, "Pod") := {} if {
  true
}

pod_template_metadata_for_kind(obj, "CronJob") := meta if {
  meta := object.get(obj, ["spec", "jobTemplate", "spec", "template", "metadata"], {})
}

pod_template_metadata_for_kind(obj, kind) := meta if {
  kind != "Pod"
  kind != "CronJob"
  meta := object.get(obj, ["spec", "template", "metadata"], {})
}

# Effective metadata for annotation resolution from input.review.object.
# For Pods: uses the pod's own metadata.
# For controllers (Deployment, etc.): uses ONLY the pod template's metadata
# (spec.template.metadata), NOT the controller's top-level metadata.
# This prevents false positives where a misplaced annotation (e.g.
# container.apparmor.security.beta.kubernetes.io/<name>) on the controller's
# top-level metadata causes a denial even though the pods would never carry it.
effective_metadata := meta if {
  obj := object.get(input.review, "object", {})
  kind := object.get(obj, "kind", "")
  kind == "Pod"
  meta := object.get(obj, "metadata", {})
}

effective_metadata := meta if {
  obj := object.get(input.review, "object", {})
  kind := object.get(obj, "kind", "")
  kind != "Pod"
  workload_kind(kind)
  meta := pod_template_metadata_for_kind(obj, kind)
}

# Fallback for unknown kinds: use the object's own metadata, so annotations on
# an unrecognised object are not silently dropped. This mirrors the pod_spec
# and object_labels fallbacks for unknown kinds.
effective_metadata := meta if {
  obj := object.get(input.review, "object", {})
  kind := object.get(obj, "kind", "")
  kind != "Pod"
  not workload_kind(kind)
  meta := object.get(obj, "metadata", {})
}

# =============================================================================
# Controller detection helper
# =============================================================================
# Returns true when the current review object is a pod-creating controller
# (Deployment, StatefulSet, DaemonSet, ReplicaSet, ReplicationController, Job,
# CronJob) — i.e. NOT a Pod. This lets constraint Rego code differentiate
# between Pods (where Kubernetes mutations have already been applied) and
# controllers (where the pod template is checked pre-mutation).
#
# Use case: when a mutation-sensitive field (resources, runAsUser, seccompProfile,
# etc.) is ABSENT from a controller's pod template, Kubernetes admission
# controllers (LimitRange, PodSecurity, ServiceAccount admission) may still
# inject it at Pod creation time. Constraints can use `is_controller` to skip
# violations for absent fields, avoiding false positives on controllers.
# =============================================================================
is_controller if {
  obj := object.get(input.review, "object", {})
  kind := object.get(obj, "kind", "")
  kind != "Pod"
  workload_kind(kind)
}
