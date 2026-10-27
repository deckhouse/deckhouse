# =============================================================================
# Library: lib.check_path
# =============================================================================
# HostPath validator with readOnly cross-check and SPE support.
#
# Usage:
# - check_hostpath_allowed(volume, allowed_paths, containers, spe_allowed_paths)
# allowed_paths items: {pathPrefix, readOnly}
# spe_allowed_paths items: {path, readOnly, pathType}
# Returns: {"allowed": bool, "msg": string, "detail": object}
# =============================================================================
package lib.check_path

import data.lib.path.path_matches

check_hostpath_allowed(volume, allowed_paths, containers, spe_allowed_paths) := result if {
  input_hostpath_allowed(allowed_paths, volume, containers)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_hostpath_allowed(volume, allowed_paths, containers, spe_allowed_paths) := result if {
  not input_hostpath_allowed(allowed_paths, volume, containers)
  input_hostpath_allowed_exact(spe_allowed_paths, volume, containers)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_hostpath_allowed(volume, allowed_paths, containers, spe_allowed_paths) := result if {
  not input_hostpath_allowed(allowed_paths, volume, containers)
  not input_hostpath_allowed_exact(spe_allowed_paths, volume, containers)
  msg := sprintf("HostPath %v is not allowed", [volume.hostPath.path])
  result := {
    "allowed": false,
    "msg": msg,
    "detail": {
      "field": "hostPath.path",
      "actual": volume.hostPath.path,
      "policy_allowed": allowed_paths,
      "spe_applied": count(spe_allowed_paths) > 0,
      "spe_allowed": spe_allowed_paths,
    }
  }
}

input_hostpath_allowed(allowed_paths, volume, containers) if {
  allowedHostPath := allowed_paths[_]
  path_matches(allowedHostPath.pathPrefix, volume.hostPath.path)
  not allowedHostPath.readOnly == true
}

input_hostpath_allowed(allowed_paths, volume, containers) if {
  allowedHostPath := allowed_paths[_]
  path_matches(allowedHostPath.pathPrefix, volume.hostPath.path)
  allowedHostPath.readOnly
  not writeable_input_volume_mounts(volume.name, containers)
}

writeable_input_volume_mounts(volume_name, containers) if {
  container := containers[_]
  mount := container.volumeMounts[_]
  mount.name == volume_name
  not mount.readOnly
}

# An exception with readOnly: false, the CRD default, allows the path with any
# mounts. An exception with readOnly: true allows it only when the volume is
# read-only in every container.
input_hostpath_allowed_exact(allowedPaths, volume, containers) if {
  allowedHostPath := allowedPaths[_]
  spe_path_matches(allowedHostPath, volume.hostPath.path)
  volume_mount_readonly_allowed(object.get(allowedHostPath, "readOnly", false), volume.name, containers)
}

# pathType follows the Ingress path types. Exact, the CRD default, allows only
# the exact path. Prefix also allows every path nested under it, compared
# segment by segment: /var/lib allows /var/lib/docker but not /var/libvirt.
# Any other value matches nothing.
spe_path_matches(allowedHostPath, path) if {
  object.get(allowedHostPath, "pathType", "Exact") == "Exact"
  allowedHostPath.path == path
}

spe_path_matches(allowedHostPath, path) if {
  allowedHostPath.pathType == "Prefix"
  path_matches(allowedHostPath.path, path)
}

volume_mount_readonly_allowed(false, volume_name, containers)

volume_mount_readonly_allowed(true, volume_name, containers) if {
  get_volume_mount_readonly(volume_name, containers)
}

# A volume is read-only only when every container mounts it read-only. A volume
# mounted read-only in one container and writable in another is writable.
get_volume_mount_readonly(volume_name, containers) := true if {
  volume_mount_exists(volume_name, containers)
  not writeable_input_volume_mounts(volume_name, containers)
}

get_volume_mount_readonly(volume_name, containers) := false if {
  writeable_input_volume_mounts(volume_name, containers)
}

get_volume_mount_readonly(volume_name, containers) := false if {
  not volume_mount_exists(volume_name, containers)
}

volume_mount_exists(volume_name, containers) if {
  container := containers[_]
  mount := container.volumeMounts[_]
  mount.name == volume_name
}
