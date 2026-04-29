# =============================================================================
# Library: lib.common
# =============================================================================
# Container iteration, field access helpers, and exception label utilities.
# =============================================================================
package lib.common

# Backwards-compatible container iterator (uses input.review)
input_containers contains c if {
  c := input.review.object.spec.containers[_]
}

input_containers contains c if {
  c := input.review.object.spec.initContainers[_]
}

input_containers contains c if {
  c := input.review.object.spec.ephemeralContainers[_]
}

# Parameterized container iterator (uses passed object)
input_containers_from(obj) := containers if {
  base := object.get(obj, ["spec", "containers"], [])
  init := object.get(obj, ["spec", "initContainers"], [])
  eph := object.get(obj, ["spec", "ephemeralContainers"], [])
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
  key := sprintf("security.deckhouse.io/security-policy-exception/%v", [container.name])
  label := input.review.object.metadata.labels[key]
  label != ""
} else := label if {
  key := sprintf("security.deckhouse.io/security-policy-exception/%v", [container.name])
  object.get(input.review.object.metadata.labels, key, "") == ""
  label := object.get(input.review.object.metadata.labels, "security.deckhouse.io/security-policy-exception", "")
  label != ""
} else := "" if {
  true
}

# Parameterized exception label lookup (uses labels map)
get_exception_label_from_labels(container, labels) := label if {
  key := sprintf("security.deckhouse.io/security-policy-exception/%v", [container.name])
  label := object.get(labels, key, "")
  label != ""
} else := label if {
  key := sprintf("security.deckhouse.io/security-policy-exception/%v", [container.name])
  object.get(labels, key, "") == ""
  label := object.get(labels, "security.deckhouse.io/security-policy-exception", "")
  label != ""
} else := "" if {
  true
}
