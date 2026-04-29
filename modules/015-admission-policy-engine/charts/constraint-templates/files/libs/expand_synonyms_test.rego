package lib.expand_synonyms_test

import data.lib.expand_synonyms

translation := {
  "RuntimeDefault": ["runtime/default"],
  "Unconfined": ["unconfined"],
  "localhost": ["Localhost"],
}

# Expand with translation table

test_expand_synonyms if {
  result := expand_synonyms.expand(["RuntimeDefault"], translation)
  result["runtime/default"]
}

# Normalize

test_normalize if {
  result := expand_synonyms.normalize("RuntimeDefault", translation)
  result == "runtime/default"
}
