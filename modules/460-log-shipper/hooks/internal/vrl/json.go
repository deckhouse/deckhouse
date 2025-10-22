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

// ParseJSONRule provides the message data as an object for future modifications/validations.
// Parsed data will be equal to message to simplify further transformations, e.g., log filtration's.
//
// It is usually used in a combination with other rules.
const ParseJSONRule Rule = `
if !exists(.parsed_data) {
    structured, err = parse_json(.message)
    if err == null {
        .parsed_data = structured
    } else {
        .parsed_data = .message
    }
}
`
