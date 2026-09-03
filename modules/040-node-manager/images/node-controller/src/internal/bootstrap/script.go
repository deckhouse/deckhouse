/*
Copyright 2026 Flant JSC

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

package bootstrap

import (
	"bytes"
	"fmt"
	"regexp"
	"text/template"

	"github.com/deckhouse/node-controller/internal/controller/nodegroup/machineclass"
)

const (
	libPath           = "candi/bashible/lib.sh.tpl"
	prerequisitesPath = "candi/bashible/bootstrap/01-bootstrap-prerequisites.sh.tpl"
)

// Kept from helm verbatim. It never matches, because the prerequisites template
// opens with a comment action and so renders a leading newline; dropping it
// here would change the script the moment that comment goes away.
var shebangRe = regexp.MustCompile(`^#!/bin/bash\nset -Eeo pipefail\n`)

// The literal head and tail of helm's define "bootstrap_script"
// (templates/node-group/_bootstrap.tpl). The leading and missing trailing
// newlines are load-bearing: helm's whitespace trimming puts them exactly here.
const (
	scriptHeader = `
#!/usr/bin/env bash
set -Eeuo pipefail
BOOTSTRAP_DIR="/var/lib/bashible"
TMPDIR="/opt/deckhouse/tmp"
mkdir -p "${BOOTSTRAP_DIR}" "${TMPDIR}"
export PATH="/opt/deckhouse/bin:/usr/local/bin:$PATH"
bootstrap_log_init() {
  if [[ -z ${BOOTSTRAP_LOG:-} ]]; then
    mkdir -p /var/log/d8/bashible
    exec {stdout_fd}>&1
    exec > >(tee -a /var/log/d8/bashible/bootstrap.log >&${stdout_fd}) 2>&1
    export BOOTSTRAP_LOG=1
  fi
}
bootstrap_log_init`

	tailLogSnippet = `
/opt/deckhouse/bin/tail-log ${TMPDIR}/bootstrap.log &
log_pid=$!`

	scriptFooter = `
get_phase2 | bash
if [ -n "${log_pid:-}" ]; then
  kill -9 "${log_pid}"
fi`
)

// RenderScript renders /var/lib/bashible/bootstrap.sh, the script both the
// cloud-init and the manual bootstrap flows put on a node. It is a port of the
// helm define "bootstrap_script" (templates/node-group/_bootstrap.tpl).
func RenderScript(in Input) ([]byte, error) {
	ctx := in.templateContext()

	lib := in.Files.Get(libPath)
	if lib == "" {
		return nil, fmt.Errorf("read %s: not among the bootstrap templates", libPath)
	}
	phase2, err := renderTemplate("lib", lib+"\n{{ template \"get-phase2\" $ }}", ctx)
	if err != nil {
		return nil, fmt.Errorf("render bashible library: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString(scriptHeader)
	buf.Write(phase2)

	if prereq := in.Files.Get(prerequisitesPath); prereq != "" {
		rendered, err := renderTemplate("prerequisites", prereq, ctx)
		if err != nil {
			return nil, fmt.Errorf("render bootstrap prerequisites: %w", err)
		}
		buf.WriteString("\n")
		buf.WriteString(shebangRe.ReplaceAllString(string(rendered), ""))
	}

	nodeType, _ := in.NodeGroup["nodeType"].(string)
	_, hasStaticInstances := in.NodeGroup["staticInstances"]
	if nodeType == "CloudEphemeral" || hasStaticInstances {
		buf.WriteString(tailLogSnippet)
	}
	buf.WriteString(scriptFooter)

	return buf.Bytes(), nil
}

func renderTemplate(name, text string, data any) ([]byte, error) {
	t, err := template.New(name).Funcs(templateFuncs(name)).Parse(text)
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}

	var out bytes.Buffer
	if err := t.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("execute template %s: %w", name, err)
	}
	return out.Bytes(), nil
}

// templateFuncs is the function set the bashible templates were written
// against: helm's sprig plus a tpl that recurses the way helm's own does. The
// parent name travels into the nested render so a failure names the template
// that broke, not just "tpl".
func templateFuncs(parent string) template.FuncMap {
	funcs := machineclass.FuncMap()
	funcs["tpl"] = func(text string, data any) (string, error) {
		out, err := renderTemplate(parent+" tpl", text, data)
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
	// machineclass leaves an include stub whose message names its own renderer.
	funcs["include"] = func(name string, _ any) (string, error) {
		return "", fmt.Errorf("include %q is not available in the bootstrap renderer", name)
	}
	return funcs
}
