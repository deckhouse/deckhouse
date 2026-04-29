package lib.check_bool_test

import data.lib.check_bool

# Field matches expected value → allowed
test_container_bool_matches_expected if {
  container := {"securityContext": {"privileged": false}}
  result := check_bool.check_container_bool(
    container,
    ["securityContext", "privileged"],
    "privileged",
    false,
    false,
    ["spec", "securityContext", "privileged", "allowedValue"],
    {},
    "default"
  )
  result.allowed == true
  result.detail == {}
}

# Field does not match expected → not allowed
test_container_bool_violates if {
  container := {"securityContext": {"privileged": true}}
  result := check_bool.check_container_bool(
    container,
    ["securityContext", "privileged"],
    "privileged",
    false,
    false,
    ["spec", "securityContext", "privileged", "allowedValue"],
    {},
    "default"
  )
  result.allowed == false
  contains(result.msg, "privileged has value true, expected false")
  not contains(result.msg, "SPE allows")
  result.detail.msg == "privileged has value true, expected false."
  result.detail.spe_applied == false
}

# Default matches expected → allowed
test_container_bool_default_matches if {
  container := {}
  result := check_bool.check_container_bool(
    container,
    ["securityContext", "privileged"],
    "privileged",
    false,
    false,
    ["spec", "securityContext", "privileged", "allowedValue"],
    {},
    "default"
  )
  result.allowed == true
  result.detail == {}
}

# Default does not match expected → not allowed
test_container_bool_default_violates if {
  container := {}
  result := check_bool.check_container_bool(
    container,
    ["securityContext", "allowPrivilegeEscalation"],
    "allowPrivilegeEscalation",
    false,
    true,
    ["spec", "securityContext", "allowPrivilegeEscalation", "allowedValue"],
    {},
    "default"
  )
  result.allowed == false
  result.detail.msg == "allowPrivilegeEscalation has value true, expected false."
  result.detail.spe_applied == false
}

# SPE allows actual value
test_container_bool_spe_allows if {
  container := {"name": "app", "securityContext": {"privileged": true}}
  labels := {"security.deckhouse.io/security-policy-exception": "spe"}
  result := check_bool.check_container_bool(
    container,
    ["securityContext", "privileged"],
    "privileged",
    false,
    false,
    ["spec", "securityContext", "privileged", "allowedValue"],
    labels,
    "default"
  ) with data.inventory as inventory_allow_true
  result.allowed == true
  result.detail == {}
}

# SPE denies value
test_container_bool_spe_denies if {
  container := {"name": "app", "securityContext": {"privileged": true}}
  labels := {"security.deckhouse.io/security-policy-exception": "spe"}
  result := check_bool.check_container_bool(
    container,
    ["securityContext", "privileged"],
    "privileged",
    false,
    false,
    ["spec", "securityContext", "privileged", "allowedValue"],
    labels,
    "default"
  ) with data.inventory as inventory_allow_false
  result.allowed == false
  contains(result.msg, "expected")
  contains(result.msg, "forbidden: true")
  contains(result.msg, "policy allows: false")
  contains(result.msg, "SPE allows: [false]")
  result.detail.msg == "privileged has value true, expected false."
  result.detail.spe_applied == true
  result.detail.forbidden == true
  result.detail.policy_allows == false
  result.detail.spe_allows == [false]
}

# Pod-level SPE denies value with context
test_pod_bool_spe_denies_with_context if {
  pod := {
    "metadata": {"labels": {"security.deckhouse.io/security-policy-exception": "spe"}, "namespace": "default"},
    "spec": {"hostPID": true}
  }
  result := check_bool.check_pod_bool(
    pod,
    ["spec", "hostPID"],
    "hostPID",
    false,
    false,
    ["spec", "network", "hostPID", "allowedValue"]
  ) with data.inventory as inventory_pod_allow_false
  result.allowed == false
  contains(result.msg, "forbidden: true")
  contains(result.msg, "policy allows: false")
  contains(result.msg, "SPE allows: false")
  result.detail.msg == "hostPID has value true, expected false."
  result.detail.spe_applied == true
  result.detail.forbidden == true
  result.detail.policy_allows == false
  result.detail.spe_allows == false
}

# Pod-level boolean with SPE
test_pod_bool_spe_allows if {
  pod := {
    "metadata": {"labels": {"security.deckhouse.io/security-policy-exception": "spe"}, "namespace": "default"},
    "spec": {"hostPID": true}
  }
  result := check_bool.check_pod_bool(
    pod,
    ["spec", "hostPID"],
    "hostPID",
    false,
    false,
    ["spec", "network", "hostPID", "allowedValue"]
  ) with data.inventory as inventory_pod_allow_true
  result.allowed == true
  result.detail == {}
}

inventory_allow_true := {
  "namespace": {
    "default": {
      "deckhouse.io/v1alpha1": {
        "SecurityPolicyException": {
          "spe": {
            "metadata": {"name": "spe"},
            "spec": {
              "securityContext": {
                "privileged": {"allowedValue": true}
              }
            }
          }
        }
      }
    }
  }
}

inventory_allow_false := {
  "namespace": {
    "default": {
      "deckhouse.io/v1alpha1": {
        "SecurityPolicyException": {
          "spe": {
            "metadata": {"name": "spe"},
            "spec": {
              "securityContext": {
                "privileged": {"allowedValue": false}
              }
            }
          }
        }
      }
    }
  }
}

inventory_pod_allow_true := {
  "namespace": {
    "default": {
      "deckhouse.io/v1alpha1": {
        "SecurityPolicyException": {
          "spe": {
            "metadata": {"name": "spe"},
            "spec": {
              "network": {
                "hostPID": {"allowedValue": true}
              }
            }
          }
        }
      }
    }
  }
}

inventory_pod_allow_false := {
  "namespace": {
    "default": {
      "deckhouse.io/v1alpha1": {
        "SecurityPolicyException": {
          "spe": {
            "metadata": {"name": "spe"},
            "spec": {
              "network": {
                "hostPID": {"allowedValue": false}
              }
            }
          }
        }
      }
    }
  }
}

