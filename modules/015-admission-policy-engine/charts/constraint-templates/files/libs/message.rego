# =============================================================================
# Library: lib.message
# =============================================================================
# Standardized violation message builder.
#
# Usage:
# - build(violation_type, subject, actual_value, allowed_by_policy, exception_info)
#   (legacy API, kept for backward compatibility)
# - violation_message(reason, context, detail)
#   detail preferred shape:
#   {
#     "field": "hostPID",
#     "actual": true,
#     "policy_allowed": false,
#     "spe_applied": true,
#     "spe_allowed": false
#   }
# =============================================================================
package lib.message

build(violation_type, subject, actual_value, allowed_by_policy, exception_info) := msg if {
  exception_info != ""
  msg := sprintf("%v: %v | %v | %v | %v", [violation_type, subject, actual_value, allowed_by_policy, exception_info])
}

build(violation_type, subject, actual_value, allowed_by_policy, exception_info) := msg if {
  exception_info == ""
  msg := sprintf("%v: %v | %v | %v", [violation_type, subject, actual_value, allowed_by_policy])
}

# The SPE-context branch comes first and the plain branch is its `else`, so
# only one of the two is ever evaluated. As two separate complete rules, OPA
# would evaluate both bodies (plus a negation) on every violation.
violation_message(reason, context, detail) := msg if {
  detail_has_spe_context(detail)
  msg := sprintf("%v, %v | %v: %v | policy allows: %v | SPE allows: %v", [
    reason,
    context,
    detail_field(detail),
    detail_actual(detail),
    detail_policy_allowed(detail),
    detail_spe_allowed(detail),
  ])
} else := msg if {
  msg := sprintf("%v, %v | %v: %v | policy allows: %v", [
    reason,
    context,
    detail_field(detail),
    detail_actual(detail),
    detail_policy_allowed(detail),
  ])
}

# Backward-compatible helpers for old check_bool detail keys.
#
# The alternative key is reached through `else` rather than through a nested
# object.get default: a default argument is evaluated eagerly, so the nested
# form always paid for both lookups.
detail_field(detail) := object.get(detail, "field", "value")

detail_actual(detail) := v if {
  v := detail.actual
} else := v if {
  v := object.get(detail, "forbidden", "unknown")
}

detail_policy_allowed(detail) := v if {
  v := detail.policy_allowed
} else := v if {
  v := object.get(detail, "policy_allows", "unknown")
}

detail_spe_allowed(detail) := v if {
  v := detail.spe_allowed
} else := v if {
  v := object.get(detail, "spe_allows", "unknown")
}

detail_has_spe_context(detail) if {
  object.get(detail, "spe_applied", false)
}
