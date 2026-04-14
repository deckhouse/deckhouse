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
