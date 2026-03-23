# =============================================================================
# Library: lib.message
# =============================================================================
# Standardized violation message builder.
#
# Usage:
# - build(violation_type, subject, actual_value, allowed_by_policy, exception_info)
# exception_info can be "" to omit the SPE part.
# =============================================================================
package lib.message

build(violation_type, subject, actual_value, allowed_by_policy, exception_info) := msg if {
  exception_info != ""
  msg := sprintf("%v: %v. | %v | %v | %v", [violation_type, subject, actual_value, allowed_by_policy, exception_info])
}

build(violation_type, subject, actual_value, allowed_by_policy, exception_info) := msg if {
  exception_info == ""
  msg := sprintf("%v: %v. | %v | %v", [violation_type, subject, actual_value, allowed_by_policy])
}
