# Rego Libraries for Constraint Templates

## Library Hierarchy

### Low-Level Primitives
| Library       | File           | Purpose                                             |
| ------------- | -------------- | --------------------------------------------------- |
| lib.common    | common.rego    | Container iterators, field access, exception labels |
| lib.exception | exception.rego | SPE resolution and allowed values extraction        |
| lib.range     | range.rego     | Numeric range checking primitives                   |
| lib.set       | set.rego       | Set membership primitives                           |
| lib.str       | str.rego       | String prefix/suffix/contains                       |
| lib.match     | match.rego     | Regex and glob matching                             |
| lib.bool      | bool.rego      | Boolean field checking primitives                   |
| lib.path      | path.rego      | Filesystem path prefix matching                     |
| lib.object    | object.rego    | Partial object matching                             |

### Higher-Level Validators
| Library                | File                    | Purpose                           | Includes SPE |
| ---------------------- | ----------------------- | --------------------------------- | ------------ |
| lib.check_bool         | check_bool.rego         | Boolean field validation          | Yes          |
| lib.check_set          | check_set.rego          | Set membership validation         | Yes          |
| lib.check_range        | check_range.rego        | Numeric range validation          | Yes          |
| lib.check_subset       | check_subset.rego       | Subset/superset validation        | Yes          |
| lib.check_object_match | check_object_match.rego | Multi-field partial matching      | Yes          |
| lib.check_path         | check_path.rego         | HostPath validation with readOnly | Yes          |
| lib.resolve_value      | resolve_value.rego      | Multi-source value resolution     | No           |
| lib.expand_synonyms    | expand_synonyms.rego    | Synonym expansion/normalization   | No           |
| lib.message            | message.rego            | Standardized violation messages   | No           |

## How to Use

1. Include the library in your constraint template `libs` section.
2. Import the needed functions.
3. Call the library function with your parameters.
4. Check the result and emit violation if not allowed.

## Testing

- Library unit tests: `cd tests/libs && opa test . -v`
- Constraint integration tests: `cd tests && ./run_all_tests.sh`
