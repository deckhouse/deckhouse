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

// LocalTimezoneRule formats all timestamps with a local timezone.
// Example: 2019-10-12T07:20:50.52Z -> 2019-10-12T09:20:50.52+02:00 for the Europe/Berlin timezone
const LocalTimezoneRule Rule = `
if exists(."timestamp") {
    ts = parse_timestamp!(."timestamp", format: "%+")
    ."timestamp" = format_timestamp!(ts, format: "%+", timezone: "local")
}

if exists(."timestamp_end") {
    ts = parse_timestamp!(."timestamp_end", format: "%+")
    ."timestamp_end" = format_timestamp!(ts, format: "%+", timezone: "local")
}
`

// DateTimeRule copies time to the datetime field. Only relevant if the Splunk destination is used.
const DateTimeRule Rule = `
if exists(."timestamp") {
  ."datetime" = ."timestamp"
}
`
