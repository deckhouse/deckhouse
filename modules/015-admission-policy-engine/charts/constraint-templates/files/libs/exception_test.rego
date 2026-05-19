package lib.exception_test

import data.lib.exception

# resolve_spe_from_labels resolves by global label

test_resolve_spe_from_labels_found if {
  labels := {"security.deckhouse.io/security-policy-exception": "spe"}
  namespace := "default"
  result := exception.resolve_spe_from_labels(labels, namespace) with data.inventory as inventory_spe
  result.metadata.name == "spe"
}

# resolve_spe_from_labels returns empty object when label missing

test_resolve_spe_from_labels_missing_label if {
  labels := {"other": "x"}
  namespace := "default"
  result := exception.resolve_spe_from_labels(labels, namespace) with data.inventory as inventory_spe
  result == {}
}

# resolve_spe_for_container resolves by container-specific label

test_resolve_spe_for_container_specific_label if {
  labels := {"security.deckhouse.io/security-policy-exception/app": "spe"}
  container := {"name": "app"}
  namespace := "default"
  result := exception.resolve_spe_for_container(container, labels, namespace) with data.inventory as inventory_spe
  result.metadata.name == "spe"
}

# resolve_spe_for_container falls back to global label

test_resolve_spe_for_container_global_fallback if {
  labels := {"security.deckhouse.io/security-policy-exception": "spe"}
  container := {"name": "app"}
  namespace := "default"
  result := exception.resolve_spe_for_container(container, labels, namespace) with data.inventory as inventory_spe
  result.metadata.name == "spe"
}

# resolve_spe_for_container returns empty when missing

test_resolve_spe_for_container_missing if {
  labels := {"other": "x"}
  container := {"name": "app"}
  namespace := "default"
  result := exception.resolve_spe_for_container(container, labels, namespace) with data.inventory as inventory_spe
  result == {}
}

# allowed_values_or_empty handles null/array/scalar

test_allowed_values_or_empty_null if {
  spe := {}
  exception.allowed_values_or_empty(spe, ["spec", "securityContext", "privileged"]) == []
}

test_allowed_values_or_empty_array if {
  spe := {"spec": {"securityContext": {"privileged": [true, false]}}}
  exception.allowed_values_or_empty(spe, ["spec", "securityContext", "privileged"]) == [true, false]
}

test_allowed_values_or_empty_scalar if {
  spe := {"spec": {"securityContext": {"privileged": true}}}
  exception.allowed_values_or_empty(spe, ["spec", "securityContext", "privileged"]) == [true]
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
                "privileged": [true]
              }
            }
          }
        }
      }
    }
  }
}
