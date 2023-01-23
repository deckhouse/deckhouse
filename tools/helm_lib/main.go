package main

import (
	"bytes"
	"os"
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

func parseFile(filename string) string {
	c, _ := os.ReadFile(filename)

	rDefine := regexp.MustCompile(`\{\{- define "([a-z_]+)"\s*-?\}\}`)
	rComment := regexp.MustCompile(`\{\{-\s*/\*(.+)\*/\s*-?\}\}`)
	rNewLine := regexp.MustCompile(`\n`)
	rUsage := regexp.MustCompile(`Usage: (.+)`)

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
	for _, path := range paths {
		res := parseFile(path)
		all = append(all, res)
	}

	a := strings.Join(all, "\n")
	err = os.WriteFile("/deckhouse/helm_lib/README.tpl.md", []byte(a), 0o644)
	if err != nil {
		panic(err)
	}
}
