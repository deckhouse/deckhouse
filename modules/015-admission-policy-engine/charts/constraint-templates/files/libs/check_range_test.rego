package lib.check_range_test

import data.lib.check_range

# Value in range

test_container_in_range_allowed if {
  container := {"securityContext": {"runAsUser": 1000}}
  result := check_range.check_container_in_range(
    container,
    ["securityContext", "runAsUser"],
    "runAsUser",
    [{"min": 1000, "max": 2000}],
    ["spec", "securityContext", "runAsUser", "allowedValues"],
    {},
    "default"
  )
  result.allowed == true
}

# Value out of range

test_container_in_range_denied if {
  container := {"securityContext": {"runAsUser": 3000}}
  result := check_range.check_container_in_range(
    container,
    ["securityContext", "runAsUser"],
    "runAsUser",
    [{"min": 1000, "max": 2000}],
    ["spec", "securityContext", "runAsUser", "allowedValues"],
    {},
    "default"
  )
  result.allowed == false
  contains(result.msg, "runAsUser has value 3000 which is out of allowed ranges")
  not contains(result.msg, "SPE allows")
}

# Boundary

test_container_in_range_boundary if {
  container := {"securityContext": {"runAsUser": 1000}}
  result := check_range.check_container_in_range(
    container,
    ["securityContext", "runAsUser"],
    "runAsUser",
    [{"min": 1000, "max": 2000}],
    ["spec", "securityContext", "runAsUser", "allowedValues"],
    {},
    "default"
  )
  result.allowed == true
}

# SPE allows

test_container_in_range_spe_allows if {
  container := {"name": "app", "securityContext": {"runAsUser": 3000}}
  labels := {"security.deckhouse.io/security-policy-exception": "spe"}
  result := check_range.check_container_in_range(
    container,
    ["securityContext", "runAsUser"],
    "runAsUser",
    [{"min": 1000, "max": 2000}],
    ["spec", "securityContext", "runAsUser", "allowedValues"],
    labels,
    "default"
  ) with data.inventory as inventory_spe
  result.allowed == true
}

# SPE present but does not allow value -> denied with SPE context

test_container_in_range_spe_denied_with_context if {
  container := {"name": "app", "securityContext": {"runAsUser": 3000}}
  labels := {"security.deckhouse.io/security-policy-exception": "spe_other"}
  result := check_range.check_container_in_range(
    container,
    ["securityContext", "runAsUser"],
    "runAsUser",
    [{"min": 1000, "max": 2000}],
    ["spec", "securityContext", "runAsUser", "allowedValues"],
    labels,
    "default"
  ) with data.inventory as inventory_spe
  result.allowed == false
  contains(result.msg, "forbidden: 3000")
  contains(result.msg, "SPE allows: [2500]")
}

# Ports in range

test_ports_in_ranges if {
  result := check_range.check_ports_in_ranges([80, 443], "hostPorts", [{"min": 1, "max": 65535}], [])
  result.allowed == true
}

inventory_spe := {
  "namespace": {
    "default": {
      "deckhouse.io/v1alpha1": {
        "SecurityPolicyException": {
          "spe": {
            "metadata": {"name": "spe"},
            "spec": {
              "securityContext": {
                "runAsUser": {"allowedValues": [3000]}
              }
            }
          },
          "spe_other": {
            "metadata": {"name": "spe_other"},
            "spec": {
              "securityContext": {
                "runAsUser": {"allowedValues": [2500]}
              }
            }
          }
        }
      }
    }
  }
}
