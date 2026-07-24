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
  key := sprintf("security.deckhouse.io/security-policy-exception.container.%v", [container.name])
  label := input.review.object.metadata.labels[key]
  label != ""
} else := label if {
  key := sprintf("security.deckhouse.io/security-policy-exception.container.%v", [container.name])
  object.get(input.review.object.metadata.labels, key, "") == ""
  label := object.get(input.review.object.metadata.labels, "security.deckhouse.io/security-policy-exception", "")
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
