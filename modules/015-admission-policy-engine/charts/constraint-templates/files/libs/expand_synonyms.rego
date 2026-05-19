# =============================================================================
# Library: lib.expand_synonyms
# =============================================================================
# Synonym expansion and normalization helpers.
#
# Usage:
# - expand(values, translation_table) expands values with synonyms.
# - normalize(value, translation_table) returns canonical value.
# translation_table example: {"alias": ["canonical"], "localhost": ["localhost/*", "localhost/"]}
# =============================================================================
package lib.expand_synonyms

expand(values, translation_table) := expanded if {
  expanded := {v | v := values[_]} | {syn | v := values[_]; syn := translation_table[v][_]} | localhost_wildcards(values, translation_table)
}

normalize(value, translation_table) := out if {
  value == null
  out := null
}

normalize(value, translation_table) := out if {
  value != null
  normalized := translation_table[value][0]
  out := normalized
}

normalize(value, translation_table) := out if {
  value != null
  not translation_table[value]
  out := value
}

localhost_wildcards(values, translation_table) := out if {
  out := {translation_table.localhost[_] | v := values[_]; startswith(v, "localhost/")}
}
