package lib.resolve_value_test

import data.lib.resolve_value

# First source found

test_resolve_from_sources_first if {
  obj := {"spec": {"securityContext": {"seccompProfile": {"type": "RuntimeDefault"}}}}
  container := {"securityContext": {"seccompProfile": {"type": "Localhost"}}}
  sources := [
    {"type": "container_field", "path": ["securityContext", "seccompProfile", "type"]},
    {"type": "pod_field", "path": ["spec", "securityContext", "seccompProfile", "type"]}
  ]
  result := resolve_value.resolve_from_sources(obj, container, sources)
  result == "Localhost"
}

# Fallback to second source

test_resolve_from_sources_fallback if {
  obj := {"spec": {"securityContext": {"seccompProfile": {"type": "RuntimeDefault"}}}}
  container := {}
  sources := [
    {"type": "container_field", "path": ["securityContext", "seccompProfile", "type"]},
    {"type": "pod_field", "path": ["spec", "securityContext", "seccompProfile", "type"]}
  ]
  result := resolve_value.resolve_from_sources(obj, container, sources)
  result == "RuntimeDefault"
}

# Annotation source

test_resolve_from_sources_annotation if {
  obj := {"metadata": {"annotations": {"seccomp.security.alpha.kubernetes.io/pod": "runtime/default"}}}
  container := {}
  sources := [
    {"type": "annotation", "key": "seccomp.security.alpha.kubernetes.io/pod"}
  ]
  result := resolve_value.resolve_from_sources(obj, container, sources)
  result == "runtime/default"
}
