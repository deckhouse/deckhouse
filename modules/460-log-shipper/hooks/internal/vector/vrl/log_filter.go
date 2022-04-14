/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vrl

// LogFilterExistsRule checks whether a label exists in the log message.
const LogFilterExistsRule Rule = `
exists(.parsed_data.%s)
`

// LogFilterDoesNotExistRule returns true if there is no label in the log message.
const LogFilterDoesNotExistRule Rule = `
!exists(.parsed_data.%s)
`

// LogFilterInRule checks that the provided label value is in the following list.
const LogFilterInRule Rule = `
if is_boolean(.parsed_data.%s) || is_float(.parsed_data.%s) {
    data, err = to_string(.parsed_data.%s);
    if err != null {
        false;
    } else {
        includes(%s, data);
    };
} else {
    includes(%s, .parsed_data.%s);
}
`

// LogFilterNotInRule checks that the provided label value is out of the following list.
const LogFilterNotInRule Rule = `
if is_boolean(.parsed_data.%s) || is_float(.parsed_data.%s) {
    data, err = to_string(.parsed_data.%s);
    if err != null {
        true;
    } else {
        !includes(%s, data);
    };
} else {
    !includes(%s, .parsed_data.%s);
}
`

// LogFilterRegexSingleRule checks that a particular label value matches the provided regex.
const LogFilterRegexSingleRule Rule = `
match!(.parsed_data.%s, r'%s')
`

// LogFilterNotRegexSingleRule validates that a particular label does not match the provided regex.
const LogFilterNotRegexSingleRule Rule = `
{
  matched, err = match(.parsed_data.%s, r'%s')
  if err != null {
    true
  } else {
    !matched
  }
}
`

// LogFilterNotRegexParentRule is a wrapper around negative regex checks.
// It ensures that the label exists and the match function can be applied to it.
const LogFilterNotRegexParentRule Rule = `
if exists(.parsed_data.%s) && is_string(.parsed_data.%s) {
  %s
} else {
  true
}
`
