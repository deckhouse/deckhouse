/*
Copyright 2023 Flant JSC

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

// CEFNameAndSeverity sets default values for cef encoding.
// If also maps falco priority values to severity to make it possible to use for cef.
const CEFNameAndSeverity Rule = `
if !exists(.cef) {
  .cef = {};
};

if !exists(.cef.name) {
  .cef.name = "Deckhouse Event";
};

if !exists(.cef.severity) {
  .cef.severity = "5";
} else if is_string(.cef.severity) {
  if .cef.severity == "Debug" {
    .cef.severity = "0";
  };
  if .cef.severity == "Informational" {
    .cef.severity = "3";
  };
  if .cef.severity == "Notice" {
    .cef.severity = "4";
  };
  if .cef.severity == "Warning" {
    .cef.severity = "6";
  };
  if .cef.severity == "Error" {
    .cef.severity = "7";
  };
  if .cef.severity == "Critical" {
    .cef.severity = "8";
  };
  if .cef.severity == "Emergency" {
    .cef.severity = "10";
  };
};

`
