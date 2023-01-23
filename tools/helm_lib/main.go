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

package main

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

func reverse(ss []string) {
	last := len(ss) - 1
	for i := 0; i < len(ss)/2; i++ {
		ss[i], ss[last-i] = ss[last-i], ss[i]
	}
}

var (
	rDefine  = regexp.MustCompile(`\{\{- define "([a-z_]+)"\s*-?\}\}`)
	rComment = regexp.MustCompile(`\{\{-?\s*/\*(.+)\*/\s*-?\}\}`)
	rNewLine = regexp.MustCompile(`\n`)
	rUsage   = regexp.MustCompile(`Usage: (.+)`)
)

func parseFile(filename string) string {
	c, _ := os.ReadFile(filename)

	strs := rNewLine.Split(string(c), -1)

	definitionTemplate := `
## {{ .name }}
{{- range $i, $d := .description }}
{{ $d }}
{{- end }}

### Usage
` + "`" + "{{ .usage }}" + "`" + `

{{- if .args }}
### Arguments
{{- if .argsDesc }}
{{ .argsDesc }}
{{- end }}
{{- range $i, $a := .args }}
- {{ $a }}
{{- end }}
{{- end }}
`

	tmp := template.New("definition")
	tmp, err := tmp.Parse(definitionTemplate)
	if err != nil {
		panic(err)
	}

	all := make([]string, 0)

	for indx, str := range strs {
		defineNameMatch := rDefine.FindStringSubmatch(str)
		if defineNameMatch != nil {
			name := defineNameMatch[1]
			usage := ""
			description := make([]string, 0)
			args := make([]string, 0)

			commentIndx := indx - 1
			for ; commentIndx >= 0; commentIndx-- {
				comment := rComment.FindStringSubmatch(strs[commentIndx])
				if comment == nil {
					break
				}

				usageMatch := rUsage.FindStringSubmatch(comment[1])
				if usageMatch != nil {
					usage = usageMatch[1]
					continue
				}

				description = append(description, comment[1])
			}

			argsIndx := indx + 1
			for ; argsIndx < len(strs); argsIndx++ {
				arg := rComment.FindStringSubmatch(strs[argsIndx])
				if arg == nil {
					break
				}

				args = append(args, arg[1])
			}

			// skip internal definitions
			if len(description) == 0 {
				continue
			}

			reverse(description)

			argsDesc := ""
			if len(args) > 1 {
				argsDesc = "list:"
			}

			var tpl bytes.Buffer
			err = tmp.Execute(&tpl, map[string]interface{}{
				"name":        name,
				"usage":       usage,
				"args":        args,
				"argsDesc":    argsDesc,
				"description": description,
			})
			if err != nil {
				panic(err)
			}

			all = append(all, tpl.String())
		}
	}

	return strings.Join(all, "\n")
}

func main() {
	paths, err := filepath.Glob("/deckhouse/helm_lib/templates/*.tpl")
	if err != nil {
		panic(err)
	}

	all := make([]string, 0)
	all = append(all, "Helm utils template definitions for Deckhouse modules.", "\n")
	for _, p := range paths {
		res := parseFile(p)
		if res == "" {
			continue
		}

		base := path.Base(p)
		base = strings.Replace(base, "_", " ", -1)
		base = strings.TrimSpace(base)
		base = strings.Split(base, ".")[0]
		base = strings.Title(base)
		all = append(all, "# "+base, res)
	}

	a := strings.Join(all, "\n")
	err = os.WriteFile("/deckhouse/helm_lib/README.md", []byte(a), 0o644)
	if err != nil {
		panic(err)
	}
}
