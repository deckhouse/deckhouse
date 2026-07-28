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

pod_spec := {} if {
  obj := object.get(input.review, "object", {})
  kind := object.get(obj, "kind", "")
  not workload_kind(kind)
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

# Merge two label maps; values from override take precedence.
merge_labels(base, override) := merged if {
  all_keys := {k | base[k]} | {k | override[k]}
  merged := {k: v |
    k := all_keys[_]
    v := merge_label_value(base, override, k)
  }
}

# Override takes precedence
merge_label_value(base, override, k) := v if {
  override[k]
  v := override[k]
}

# Fall back to base when override doesn't have the key
merge_label_value(base, override, k) := v if {
  not override[k]
  v := base[k]
}

# Effective labels for SPE resolution from input.review.object.
# Merges top-level metadata.labels with pod template metadata.labels.
# Pod template labels take precedence.
object_labels := labels if {
  obj := object.get(input.review, "object", {})
  top := object.get(obj, ["metadata", "labels"], {})
  tmpl := pod_template_metadata_labels
  labels := merge_labels(top, tmpl)
}

# Effective namespace for SPE resolution from input.review.object.
object_namespace := ns if {
  obj := object.get(input.review, "object", {})
  ns := object.get(obj, ["metadata", "namespace"], "")
}

# Effective labels for SPE resolution from a given object.
# Used by library functions that receive obj as a parameter.
effective_labels(obj) := labels if {
  top := object.get(obj, ["metadata", "labels"], {})
  kind := object.get(obj, "kind", "")
  tmpl := effective_pod_template_labels_for_kind(obj, kind)
  labels := merge_labels(top, tmpl)
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
# Merges top-level metadata with pod template metadata.
# Pod template metadata takes precedence (annotations on pods created by controllers
# should be on the pod template, not the controller's top-level metadata).
effective_metadata := meta if {
  obj := object.get(input.review, "object", {})
  top := object.get(obj, "metadata", {})
  tmpl := pod_template_metadata
  meta := merge_metadata(top, tmpl)
}

merge_metadata(base, override) := merged if {
  all_keys := {k | base[k]} | {k | override[k]}
  merged := {k: v |
    k := all_keys[_]
    v := merge_label_value(base, override, k)
  }
}
