// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build ignore
// +build ignore

package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"text/template"
)

type data struct {
	Types   string
	Methods []method
}

type method struct {
	Type string
	Name string
}

func main() {
	filename := "parse.generated.go"

	var d data
	flag.StringVar(&d.Types, "types", "", "Type(s) to generate for, e.g. -types Dog,Cat,Horse")
	flag.Parse()

	types := strings.Split(d.Types, ",")
	methods := make([]method, len(types))
	for i, t := range types {
		methods[i] = method{
			Type: t,
			Name: "Parse" + strings.Title(t) + "Slice",
		}
	}

	t := template.Must(template.New("parser").Parse(parsersTemplate))

	out, err := os.Create(filename)
	if err != nil {
		log.Fatalf("cannot create file %q: %v", filename, err)
	}
	defer out.Close()
	d.Methods = methods
	t.Execute(out, d)
}

var parsersTemplate = `/*
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

// DO NOT EDIT
// This file was generated automatically with
// 	go run gen_parse.go -types {{.Types}}
//
// It is used to cast slices of snapshot types. See file types.go

package snapshot

import (
	"fmt"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)
{{- range $m := .Methods }}

// {{ $m.Name }} parses {{ $m.Type }} slice from snapshots
func {{ $m.Name }}(rs []sdkpkg.Snapshot) ([]{{ $m.Type }}, error) {
	ret := make([]{{ $m.Type }}, 0, len(rs))
	for snap, err := range sdkobjectpatch.SnapshotIter[{{ $m.Type }}](rs) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over snapshots - failed to parse {{ $m.Type }}: %w", err)	
		}
			
		ret = append(ret, snap)
	}
	return ret, nil
}
{{- end }}
`
