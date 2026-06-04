package lib.check_subset_test

import data.lib.check_subset

# Subset match

test_subset_allowed if {
  container := {"securityContext": {"capabilities": {"add": ["NET_ADMIN"]}}}
  result := check_subset.check_container_subset(
    container,
    ["securityContext", "capabilities", "add"],
    "capabilities.add",
    ["NET_ADMIN", "NET_BIND_SERVICE"],
    ["spec", "securityContext", "capabilities", "allowedValues", "add"],
    {},
    "default",
    {"case_insensitive": true}
  )
  result.allowed == true
}

# Subset violation

test_subset_denied if {
  container := {"securityContext": {"capabilities": {"add": ["SYS_ADMIN"]}}}
  result := check_subset.check_container_subset(
    container,
    ["securityContext", "capabilities", "add"],
    "capabilities.add",
    ["NET_ADMIN"],
    ["spec", "securityContext", "capabilities", "allowedValues", "add"],
    {},
    "default",
    {"case_insensitive": true}
  )
  result.allowed == false
  result.msg == "capabilities.add must be subset of [\"NET_ADMIN\"]"
  result.detail.field == "capabilities.add"
  result.detail.actual == ["SYS_ADMIN"]
  result.detail.policy_allowed == ["NET_ADMIN"]
  result.detail.spe_applied == false
  result.detail.spe_allowed == []
}

# Superset match

test_superset_allowed if {
  container := {"securityContext": {"capabilities": {"drop": ["ALL", "NET_RAW"]}}}
  result := check_subset.check_container_superset(
    container,
    ["securityContext", "capabilities", "drop"],
    "capabilities.drop",
    ["ALL"],
    ["spec", "securityContext", "capabilities", "allowedValues", "drop"],
    {},
    "default",
    {"case_insensitive": true}
  )
  result.allowed == true
}

# Superset violation

test_superset_denied if {
  container := {"securityContext": {"capabilities": {"drop": ["NET_RAW"]}}}
  result := check_subset.check_container_superset(
    container,
    ["securityContext", "capabilities", "drop"],
    "capabilities.drop",
    ["ALL"],
    ["spec", "securityContext", "capabilities", "allowedValues", "drop"],
    {},
    "default",
    {"case_insensitive": true}
  )
  result.allowed == false
  result.msg == "capabilities.drop must contain [\"ALL\"]"
  result.detail.field == "capabilities.drop"
  result.detail.actual == ["NET_RAW"]
  result.detail.policy_allowed == ["ALL"]
  result.detail.spe_applied == false
  result.detail.spe_allowed == []
}
