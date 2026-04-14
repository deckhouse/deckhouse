package lib.check_set_test

import data.lib.check_set

# Value in set

test_container_value_in_set_allowed if {
  container := {"securityContext": {"procMount": "Default"}}
  result := check_set.check_container_value_in_set(
    container,
    ["securityContext", "procMount"],
    "procMount",
    ["Default", "Unmasked"],
    ["spec", "securityContext", "procMount", "allowedValues"],
    {},
    "default"
  )
  result.allowed == true
}

# Value not in set

test_container_value_in_set_denied if {
  container := {"securityContext": {"procMount": "Masked"}}
  result := check_set.check_container_value_in_set(
    container,
    ["securityContext", "procMount"],
    "procMount",
    ["Default"],
    ["spec", "securityContext", "procMount", "allowedValues"],
    {},
    "default"
  )
  result.allowed == false
  contains(result.msg, "procMount has value Masked which is not in allowed set")
  not contains(result.msg, "SPE allows")
}

# Wildcard allows all

test_value_in_set_wildcard if {
  result := check_set.check_value_in_set_with_wildcards("net.ipv4.ip_forward", ["*"], [], {})
  result.allowed == true
}

# Prefix wildcard

test_value_in_set_prefix if {
  result := check_set.check_value_in_set_with_wildcards("net.ipv4.ip_forward", ["net.*"], [], {})
  result.allowed == true
}

# SPE allows

test_container_value_in_set_spe_allows if {
  container := {"name": "app", "securityContext": {"procMount": "Unmasked"}}
  labels := {"security.deckhouse.io/security-policy-exception": "spe"}
  result := check_set.check_container_value_in_set(
    container,
    ["securityContext", "procMount"],
    "procMount",
    ["Default"],
    ["spec", "securityContext", "procMount", "allowedValues"],
    labels,
    "default"
  ) with data.inventory as inventory_spe
  result.allowed == true
}

# SPE present but does not allow value -> denied with context

test_container_value_in_set_spe_denied_with_context if {
  container := {"name": "app", "securityContext": {"procMount": "Masked"}}
  labels := {"security.deckhouse.io/security-policy-exception": "spe"}
  result := check_set.check_container_value_in_set(
    container,
    ["securityContext", "procMount"],
    "procMount",
    ["Default"],
    ["spec", "securityContext", "procMount", "allowedValues"],
    labels,
    "default"
  ) with data.inventory as inventory_spe
  result.allowed == false
  contains(result.msg, "forbidden: Masked")
  contains(result.msg, "SPE allows: [\"Unmasked\"]")
}

# Allowlist/denylist

test_allowlist_denylist_forbidden if {
  result := check_set.check_allowlist_denylist("net.ipv4.ip_forward", ["net.*"], ["net.ipv4.*"], [], {})
  result.allowed == false
}

# Glob matcher

test_check_value_with_glob if {
  result := check_set.check_value_with_glob("runtime/default", ["runtime/*"], [])
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
                "procMount": {"allowedValues": ["Unmasked"]}
              }
            }
          },
        }
      }
    }
  }
}
