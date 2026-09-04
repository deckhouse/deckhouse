# =============================================================================
# Library: lib.common
# =============================================================================
# Container iteration, field access helpers, and exception label utilities.
# =============================================================================
package lib.common

# Location of the pod template within each pod-creating controller.
#
# This map is the single source of truth for controller shapes: every pod
# spec, metadata and label lookup below resolves through it, so a new
# workload kind is added in one place. A lookup miss (Pod, or an unrecognised
# kind) is what drives the `else` fallbacks below, which is why Pod is
# intentionally absent — a Pod carries its pod spec directly.
#
# The map is deliberately wider than the kinds the module's own generated
# constraints match: the `workload_kinds` Helm helper excludes ReplicaSet,
# because a ReplicaSet is created by a Deployment and its denial surfaces in
# the Deployment status instead. The library still resolves ReplicaSet so that
# a hand-written Constraint matching it behaves correctly.
#
# A map, rather than one function head per kind: benchmarked with
# tests/tools/rulebench.sh, per-kind heads cost ~25 allocations/eval more on
# controller reviews, because each definition is dispatched separately. The
# map is materialized once per evaluation instead.
pod_template_paths := {
  "Deployment": ["spec", "template"],
  "StatefulSet": ["spec", "template"],
  "DaemonSet": ["spec", "template"],
  "ReplicaSet": ["spec", "template"],
  "ReplicationController": ["spec", "template"],
  "Job": ["spec", "template"],
  "CronJob": ["spec", "jobTemplate", "spec", "template"],
}

# Current review object.
review_object := object.get(input.review, "object", {})

# Pod template of a controller. Undefined for a Pod and for an unrecognised
# kind, so callers fall through to their `else` clause.
pod_template_of(obj) := object.get(obj, pod_template_paths[object.get(obj, "kind", "")], {})

# Normalize PodSpec location across pod-creating workloads.
# - Pod: spec
# - Deployment/StatefulSet/DaemonSet/ReplicaSet/ReplicationController/Job: spec.template.spec
# - CronJob: spec.jobTemplate.spec.template.spec
# A Pod is by far the most common review object, and it carries its pod spec
# directly, so it is resolved with a plain reference and never pays for
# template resolution. Controllers fall through to pod_spec_of.
pod_spec := input.review.object.spec if {
  input.review.object.kind == "Pod"
} else := out if {
  out := pod_spec_of(review_object)
}

# Pod spec for a given object: parameterized counterpart of pod_spec, for
# library functions that receive obj as a parameter.
#
# For an unknown/absent kind, the fallback reads object.spec instead of {}.
# This prevents fail-open: if pod_spec were {}, input_containers would be
# empty and every container-level check would silently pass. For security
# modules, an unrecognised input should not mean "allowed".
pod_spec_of(obj) := out if {
  out := object.get(pod_template_of(obj), "spec", {})
} else := out if {
  out := object.get(obj, "spec", {})
}

# Object normalized to a pod-like shape: its own metadata plus the resolved
# pod spec. Lets a spec-relative path such as ["spec", "hostNetwork"] resolve
# correctly for both Pods and controllers.
normalized_pod_object(obj) := {
  "metadata": object.get(obj, "metadata", {}),
  "spec": pod_spec_of(obj),
}

# Backwards-compatible container iterator (uses input.review).
# An array rather than a set: building a set requires hashing every container
# object, and every call site only iterates.
input_containers := input_containers_from(pod_spec)

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

# Effective labels for SPE resolution from input.review.object.
# Pod fast path, as in pod_spec: a Pod's own labels are the SPE labels.
object_labels := object.get(input.review.object.metadata, "labels", {}) if {
  input.review.object.kind == "Pod"
} else := labels if {
  labels := effective_labels(review_object)
}

# Effective namespace for SPE resolution from input.review.object.
object_namespace := object.get(review_object, ["metadata", "namespace"], "")

# Effective labels for SPE resolution from a given object.
# Used by library functions that receive obj as a parameter.
# For controllers: uses ONLY the pod template's metadata.labels.
# For Pods, and as a fallback for unknown kinds, uses the object's own
# metadata.labels, so SPE labels are not silently dropped. Mirrors the
# pod_spec fallback to object.spec.
effective_labels(obj) := labels if {
  labels := object.get(pod_template_of(obj), ["metadata", "labels"], {})
} else := labels if {
  labels := object.get(obj, ["metadata", "labels"], {})
}

# Effective metadata for annotation resolution from input.review.object.
# For controllers (Deployment, etc.): uses ONLY the pod template's metadata
# (spec.template.metadata), NOT the controller's top-level metadata.
# This prevents false positives where a misplaced annotation (e.g.
# container.apparmor.security.beta.kubernetes.io/<name>) on the controller's
# top-level metadata causes a denial even though the pods would never carry it.
# For Pods, and as a fallback for unknown kinds, uses the object's own metadata.
effective_metadata := input.review.object.metadata if {
  input.review.object.kind == "Pod"
} else := meta if {
  meta := object.get(pod_template_of(review_object), "metadata", {})
} else := meta if {
  meta := object.get(review_object, "metadata", {})
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
  pod_template_paths[object.get(review_object, "kind", "")]
}
