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

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

// Rule is a representation of a VRL rule.
type Rule string

// String returns string representation of the rule.
func (r Rule) String() string {
	return strings.TrimSpace(string(r))
}

// Render returns formatted VRL rule with provided args.
func (r Rule) Render(args Args) (string, error) {
	var res bytes.Buffer

	tpl, err := template.New("vrl-transform").
		Funcs(sprig.TxtFuncMap()).
		Parse(string(r))
	if err != nil {
		return "", err
	}

	err = tpl.Execute(&res, args)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(res.String()), nil
}

// Args for rendering VRL rules.
type Args map[string]interface{}

func Combine(r1, r2 Rule) Rule {
	return Rule(strings.TrimSpace(string(r1)) + "\n\n" + strings.TrimSpace(string(r2)))
}

// FileSourceHostIPRule sets the host_ip label from the VECTOR_HOST_IP environment variable for File sources.
// This adds the node's IP address to the log metadata.
const FileSourceHostIPRule Rule = `
."host_ip" = "$VECTOR_HOST_IP"
`
