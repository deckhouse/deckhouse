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

// FilterExistsRule checks whether a label exists in the log message.
const FilterExistsRule Rule = `
exists(.{{ $.filter.Field }})
`

// FilterDoesNotExistRule returns true if there is no label in the log message.
const FilterDoesNotExistRule Rule = `
!exists(.{{ $.filter.Field }})
`

// FilterInRule checks that the provided label value is in the following list.
const FilterInRule Rule = `
if is_boolean(.{{ $.filter.Field }}) || is_float(.{{ $.filter.Field }}) {
    data, err = to_string(.{{ $.filter.Field }});
    if err != null {
        false;
    } else {
        includes({{ $.filter.Values | toJson }}, data);
    };
} else if .{{ $.filter.Field }} == null {
    false;
} else {
    includes({{ $.filter.Values | toJson }}, .{{ $.filter.Field }});
}
`

// FilterNotInRule checks that the provided label value is out of the following list.
const FilterNotInRule Rule = `
if is_boolean(.{{ $.filter.Field }}) || is_float(.{{ $.filter.Field }}) {
    data, err = to_string(.{{ $.filter.Field }});
    if err != null {
        true;
    } else {
        !includes({{ $.filter.Values | toJson }}, data);
    };
} else if .{{ $.filter.Field }} == null {
    false;
} else {
    !includes({{ $.filter.Values | toJson }}, .{{ $.filter.Field }});
}
`

// FilterRegexRule checks that a particular label matches any of provided regexes.
const FilterRegexRule Rule = `
{{ range $index, $value := $.filter.Values }}
{{- if ne $index 0 }} || {{ end }}match!(.{{ $.filter.Field }}, r'{{ $value }}')
{{- end }}
`

// FilterNotRegexRule ensures that the label exists and does not match any of provided regexes.
const FilterNotRegexRule Rule = `
if exists(.{{ $.filter.Field }}) && is_string(.{{ $.filter.Field }}) {
    matched = false
{{- range $index, $value := $.filter.Values }}
    matched{{ $index }}, err = match(.{{ $.filter.Field }}, r'{{ $value }}')
    if err != null {
        true
    }
    matched = matched || matched{{ $index }}
{{- end }}
    !matched
} else {
    true
}
`
