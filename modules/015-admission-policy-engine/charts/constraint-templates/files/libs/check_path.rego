# =============================================================================
# Library: lib.check_path
# =============================================================================
# HostPath validator with readOnly cross-check and SPE support.
#
# Usage:
# - check_hostpath_allowed(volume, allowed_paths, containers, spe_allowed_paths)
# allowed_paths/spe_allowed_paths items: {pathPrefix|path, readOnly}
# Returns: {"allowed": bool, "msg": string, "detail": object}
# =============================================================================
package lib.check_path

import data.lib.common.has_field
import data.lib.common.input_containers_from
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

input_hostpath_allowed_exact(allowedPaths, volume, containers) if {
  allowedHostPath := allowedPaths[_]
  allowedHostPath.path == volume.hostPath.path
  volume_mount_readonly_matches(allowedHostPath.readOnly, volume.name, containers)
}

volume_mount_readonly_matches(expectedReadOnly, volume_name, containers) if {
  actualReadOnly := get_volume_mount_readonly(volume_name, containers)
  actualReadOnly == expectedReadOnly
}

get_volume_mount_readonly(volume_name, containers) := readOnly if {
  container := containers[_]
  mount := container.volumeMounts[_]
  mount.name == volume_name
  has_field(mount, "readOnly")
  readOnly := mount.readOnly
}

get_volume_mount_readonly(volume_name, containers) := false if {
  container := containers[_]
  mount := container.volumeMounts[_]
  mount.name == volume_name
  not has_field(mount, "readOnly")
}

get_volume_mount_readonly(volume_name, containers) := false if {
  not volume_mount_exists(volume_name, containers)
}

volume_mount_exists(volume_name, containers) if {
  container := containers[_]
  mount := container.volumeMounts[_]
  mount.name == volume_name
}
