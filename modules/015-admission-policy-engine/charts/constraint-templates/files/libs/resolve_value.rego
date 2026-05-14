# =============================================================================
# Library: lib.resolve_value
# =============================================================================
# Multi-source value resolution with priority ordering.
#
# Usage:
# - resolve_from_sources(obj, container, sources) where sources are {type: "container_field"|"pod_field"|"annotation", path|key}
# - resolve_seccomp_profile(container, obj) or resolve_apparmor_profile(container, obj)
# Returns: {"profile": value, "location": string} for profile helpers.
# =============================================================================
package lib.resolve_value

import data.lib.common.get_field

resolve_from_sources(obj, container, sources) := out if {
  values := [v | source := sources[_]; v := resolve_from_source(obj, container, source)]
  count(values) > 0
  out := values[0]
}

resolve_from_source(obj, container, source) := out if {
  source.type == "container_field"
  out := object.get(container, source.path, null)
  out != null
}

resolve_from_source(obj, container, source) := out if {
  source.type == "pod_field"
  out := object.get(obj, source.path, null)
  out != null
}

resolve_from_source(obj, container, source) := out if {
  source.type == "annotation"
  annotations := object.get(obj, ["metadata", "annotations"], {})
  out := annotations[source.key]
  out != null
}

resolve_seccomp_profile(container, obj) := out if {
  sources := [
    {"type": "container_field", "path": ["securityContext", "seccompProfile", "type"]},
    {"type": "pod_field", "path": ["spec", "securityContext", "seccompProfile", "type"]},
    {"type": "annotation", "key": sprintf("container.seccomp.security.alpha.kubernetes.io/%v", [container.name])},
    {"type": "annotation", "key": "seccomp.security.alpha.kubernetes.io/pod"}
  ]
  value := resolve_from_sources(obj, container, sources)
  out := {"profile": value, "location": source_location(value, sources, obj, container)}
}

resolve_apparmor_profile(container, obj) := out if {
  sources := [
    {"type": "container_field", "path": ["securityContext", "appArmorProfile"]},
    {"type": "annotation", "key": sprintf("container.apparmor.security.beta.kubernetes.io/%v", [container.name])},
    {"type": "pod_field", "path": ["spec", "securityContext", "appArmorProfile"]}
  ]
  value := resolve_from_sources(obj, container, sources)
  out := {"profile": value, "location": source_location(value, sources, obj, container)}
}

source_location(value, sources, obj, container) := location if {
  source := sources[_]
  resolved := resolve_from_source(obj, container, source)
  resolved == value
  location := source_desc(source)
}

source_desc(source) := out if {
  source.type == "annotation"
  out := sprintf("annotation %v", [source.key])
}

source_desc(source) := out if {
  source.type == "container_field"
  out := sprintf("container field %v", [source.path])
}

source_desc(source) := out if {
  source.type == "pod_field"
  out := sprintf("pod field %v", [source.path])
}
