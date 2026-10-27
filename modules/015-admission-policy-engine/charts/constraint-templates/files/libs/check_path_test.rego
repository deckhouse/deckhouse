package lib.check_path_test

import data.lib.check_path

# Prefix match

test_hostpath_prefix_allowed if {
  volume := {"name": "data", "hostPath": {"path": "/var/lib"}}
  allowed := [{"pathPrefix": "/var", "readOnly": false}]
  containers := [{"volumeMounts": [{"name": "data", "readOnly": true}]}]
  result := check_path.check_hostpath_allowed(volume, allowed, containers, [])
  result.allowed == true
}

# ReadOnly check

test_hostpath_readonly_denied if {
  volume := {"name": "data", "hostPath": {"path": "/var/lib"}}
  allowed := [{"pathPrefix": "/var", "readOnly": true}]
  containers := [{"volumeMounts": [{"name": "data", "readOnly": false}]}]
  result := check_path.check_hostpath_allowed(volume, allowed, containers, [])
  result.allowed == false
  result.msg == "HostPath /var/lib is not allowed"
  result.detail.field == "hostPath.path"
  result.detail.actual == "/var/lib"
  result.detail.policy_allowed == [{"pathPrefix": "/var", "readOnly": true}]
  result.detail.spe_applied == false
  result.detail.spe_allowed == []
}

# SPE exact match

test_hostpath_spe_exact_allowed if {
  volume := {"name": "data", "hostPath": {"path": "/opt"}}
  allowed := []
  containers := [{"volumeMounts": [{"name": "data", "readOnly": false}]}]
  spe_allowed := [{"path": "/opt", "readOnly": false}]
  result := check_path.check_hostpath_allowed(volume, allowed, containers, spe_allowed)
  result.allowed == true
}

# SPE with a volume mounted read-only in one container and writable in another

test_hostpath_spe_mixed_mounts_readwrite_allowed if {
  volume := {"name": "data", "hostPath": {"path": "/opt"}}
  containers := [
    {"volumeMounts": [{"name": "data", "readOnly": true}]},
    {"volumeMounts": [{"name": "data"}]},
  ]
  spe_allowed := [{"path": "/opt", "readOnly": false}]
  result := check_path.check_hostpath_allowed(volume, [], containers, spe_allowed)
  result.allowed == true
}

test_hostpath_spe_mixed_mounts_readonly_denied if {
  volume := {"name": "data", "hostPath": {"path": "/opt"}}
  containers := [
    {"volumeMounts": [{"name": "data", "readOnly": true}]},
    {"volumeMounts": [{"name": "data", "readOnly": false}]},
  ]
  spe_allowed := [{"path": "/opt", "readOnly": true}]
  result := check_path.check_hostpath_allowed(volume, [], containers, spe_allowed)
  result.allowed == false
}

test_hostpath_spe_all_mounts_readonly_allowed if {
  volume := {"name": "data", "hostPath": {"path": "/opt"}}
  containers := [
    {"volumeMounts": [{"name": "data", "readOnly": true}]},
    {"volumeMounts": [{"name": "data", "readOnly": true}]},
  ]
  spe_allowed := [{"path": "/opt", "readOnly": true}]
  result := check_path.check_hostpath_allowed(volume, [], containers, spe_allowed)
  result.allowed == true
}

# SPE with readOnly: false, the CRD default, also covers read-only mounts

test_hostpath_spe_readwrite_covers_readonly_mounts if {
  volume := {"name": "data", "hostPath": {"path": "/opt"}}
  containers := [{"volumeMounts": [{"name": "data", "readOnly": true}]}]
  spe_allowed := [{"path": "/opt", "readOnly": false}]
  result := check_path.check_hostpath_allowed(volume, [], containers, spe_allowed)
  result.allowed == true
}

test_hostpath_spe_without_readonly_covers_readonly_mounts if {
  volume := {"name": "data", "hostPath": {"path": "/opt"}}
  containers := [{"volumeMounts": [{"name": "data", "readOnly": true}]}]
  spe_allowed := [{"path": "/opt"}]
  result := check_path.check_hostpath_allowed(volume, [], containers, spe_allowed)
  result.allowed == true
}

# SPE with pathType

test_hostpath_spe_subpath_denied_by_default if {
  volume := {"name": "data", "hostPath": {"path": "/var/lib/docker"}}
  containers := [{"volumeMounts": [{"name": "data"}]}]
  spe_allowed := [{"path": "/var/lib", "readOnly": false}]
  result := check_path.check_hostpath_allowed(volume, [], containers, spe_allowed)
  result.allowed == false
}

test_hostpath_spe_subpath_allowed if {
  volume := {"name": "data", "hostPath": {"path": "/var/lib/docker"}}
  containers := [{"volumeMounts": [{"name": "data"}]}]
  spe_allowed := [{"path": "/var/lib", "readOnly": false, "pathType": "Prefix"}]
  result := check_path.check_hostpath_allowed(volume, [], containers, spe_allowed)
  result.allowed == true
}

test_hostpath_spe_subpath_allows_the_path_itself if {
  volume := {"name": "data", "hostPath": {"path": "/var/lib/"}}
  containers := [{"volumeMounts": [{"name": "data"}]}]
  spe_allowed := [{"path": "/var/lib", "pathType": "Prefix"}]
  result := check_path.check_hostpath_allowed(volume, [], containers, spe_allowed)
  result.allowed == true
}

test_hostpath_spe_subpath_sibling_prefix_denied if {
  volume := {"name": "data", "hostPath": {"path": "/var/libvirt"}}
  containers := [{"volumeMounts": [{"name": "data"}]}]
  spe_allowed := [{"path": "/var/lib", "pathType": "Prefix"}]
  result := check_path.check_hostpath_allowed(volume, [], containers, spe_allowed)
  result.allowed == false
}

test_hostpath_spe_subpath_parent_denied if {
  volume := {"name": "data", "hostPath": {"path": "/var"}}
  containers := [{"volumeMounts": [{"name": "data"}]}]
  spe_allowed := [{"path": "/var/lib", "pathType": "Prefix"}]
  result := check_path.check_hostpath_allowed(volume, [], containers, spe_allowed)
  result.allowed == false
}

test_hostpath_spe_subpath_readonly_denied if {
  volume := {"name": "data", "hostPath": {"path": "/var/lib/docker"}}
  containers := [{"volumeMounts": [{"name": "data", "readOnly": false}]}]
  spe_allowed := [{"path": "/var/lib", "readOnly": true, "pathType": "Prefix"}]
  result := check_path.check_hostpath_allowed(volume, [], containers, spe_allowed)
  result.allowed == false
}

test_hostpath_spe_subpath_readonly_allowed if {
  volume := {"name": "data", "hostPath": {"path": "/var/lib/docker"}}
  containers := [{"volumeMounts": [{"name": "data", "readOnly": true}]}]
  spe_allowed := [{"path": "/var/lib", "readOnly": true, "pathType": "Prefix"}]
  result := check_path.check_hostpath_allowed(volume, [], containers, spe_allowed)
  result.allowed == true
}

test_hostpath_spe_root_subpaths_allowed if {
  volume := {"name": "data", "hostPath": {"path": "/etc/kubernetes"}}
  containers := [{"volumeMounts": [{"name": "data"}]}]
  spe_allowed := [{"path": "/", "pathType": "Prefix"}]
  result := check_path.check_hostpath_allowed(volume, [], containers, spe_allowed)
  result.allowed == true
}

test_hostpath_spe_exact_path_type_subpath_denied if {
  volume := {"name": "data", "hostPath": {"path": "/var/lib/docker"}}
  containers := [{"volumeMounts": [{"name": "data"}]}]
  spe_allowed := [{"path": "/var/lib", "pathType": "Exact"}]
  result := check_path.check_hostpath_allowed(volume, [], containers, spe_allowed)
  result.allowed == false
}

test_hostpath_spe_exact_path_type_allowed if {
  volume := {"name": "data", "hostPath": {"path": "/var/lib"}}
  containers := [{"volumeMounts": [{"name": "data"}]}]
  spe_allowed := [{"path": "/var/lib", "pathType": "Exact"}]
  result := check_path.check_hostpath_allowed(volume, [], containers, spe_allowed)
  result.allowed == true
}

test_hostpath_spe_unknown_path_type_denied if {
  volume := {"name": "data", "hostPath": {"path": "/var/lib"}}
  containers := [{"volumeMounts": [{"name": "data"}]}]
  spe_allowed := [{"path": "/var/lib", "pathType": "ImplementationSpecific"}]
  result := check_path.check_hostpath_allowed(volume, [], containers, spe_allowed)
  result.allowed == false
}
