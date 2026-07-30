# =============================================================================
# Library: lib.common
# =============================================================================
# Container iteration, field access helpers, and exception label utilities.
# =============================================================================
package lib.common

# Supported kinds for pod-spec extraction.
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
pod_spec := out if {
  obj := object.get(input.review, "object", {})
  kind := object.get(obj, "kind", "")
  workload_kind(kind)
  out := pod_spec_for_kind(obj, kind)
}

# For unknown/absent kind, fall back to object.spec instead of {}.
# This prevents fail-open: if pod_spec is {}, input_containers is empty
# and every container-level check silently passes.
# For security modules, an unrecognised input should not mean "allowed".
pod_spec := out if {
  obj := object.get(input.review, "object", {})
  kind := object.get(obj, "kind", "")
  not workload_kind(kind)
  out := object.get(obj, "spec", {})
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
# label may be on the controller's top-level metadata.labels OR on the pod
# template's metadata.labels (spec.template.metadata.labels). These helpers
# merge both label sets so SPE resolution works correctly for controllers.
# Pod template labels take precedence (they are the labels that will be on
# the actual pods).
# =============================================================================

# Pod template metadata labels for the current review object.
# For Pod: empty (no pod template)
# For CronJob: spec.jobTemplate.spec.template.metadata.labels
# For other controllers: spec.template.metadata.labels
pod_template_metadata_labels := labels if {
  obj := object.get(input.review, "object", {})
  kind := object.get(obj, "kind", "")
  labels := pod_template_metadata_labels_for_kind(obj, kind)
}

pod_template_metadata_labels_for_kind(obj, "Pod") := {} if {
  true
}

pod_template_metadata_labels_for_kind(obj, "CronJob") := labels if {
  labels := object.get(obj, ["spec", "jobTemplate", "spec", "template", "metadata", "labels"], {})
}

pod_template_metadata_labels_for_kind(obj, kind) := labels if {
  kind != "Pod"
  kind != "CronJob"
  labels := object.get(obj, ["spec", "template", "metadata", "labels"], {})
}

# Effective labels for SPE resolution from input.review.object.
# For Pods: uses the pod's own metadata.labels.
# For controllers (Deployment, etc.): uses ONLY the pod template's
# metadata.labels (spec.template.metadata.labels), NOT the controller's
# top-level labels. This is because SPE labels on a controller's top-level
# metadata do not propagate to the pods it creates — they only exist on
# the controller object itself. Using top-level labels would cause the
# controller to pass while the actual pods are still denied.
object_labels := labels if {
  obj := object.get(input.review, "object", {})
  kind := object.get(obj, "kind", "")
  kind == "Pod"
  labels := object.get(obj, ["metadata", "labels"], {})
}

object_labels := labels if {
  obj := object.get(input.review, "object", {})
  kind := object.get(obj, "kind", "")
  kind != "Pod"
  labels := pod_template_metadata_labels
}

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
  labels := effective_pod_template_labels_for_kind(obj, kind)
}

effective_pod_template_labels_for_kind(obj, "Pod") := {} if {
  true
}

effective_pod_template_labels_for_kind(obj, "CronJob") := labels if {
  labels := object.get(obj, ["spec", "jobTemplate", "spec", "template", "metadata", "labels"], {})
}

effective_pod_template_labels_for_kind(obj, kind) := labels if {
  kind != "Pod"
  kind != "CronJob"
  labels := object.get(obj, ["spec", "template", "metadata", "labels"], {})
}

# Pod template metadata for the current review object.
# For Pod: empty (no pod template)
# For CronJob: spec.jobTemplate.spec.template.metadata
# For other controllers: spec.template.metadata
pod_template_metadata := meta if {
  obj := object.get(input.review, "object", {})
  kind := object.get(obj, "kind", "")
  meta := pod_template_metadata_for_kind(obj, kind)
}

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
  meta := pod_template_metadata
}
