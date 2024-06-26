// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package template

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"
)

type Engine struct {
	Name string
	Data map[string]interface{}
}

// deepCopyData make a non-shallow copy of Data field
func (e Engine) deepCopyData() map[string]interface{} {
	ret := copystructure.Must(copystructure.Copy(e.Data))
	return ret.(map[string]interface{})
}

// Render
func (e Engine) Render(tmpl []byte) (out *bytes.Buffer, err error) {
	t := template.New(e.Name)
	return e.renderWithTemplate(string(tmpl), t)
}

// initFunMap creates the Engine's FuncMap and adds context-specific functions.
func (e Engine) initFunMap(t *template.Template) {
	funcMap := FuncMap()

	// include function isn't required in candi templates
	funcMap["include"] = func(name string, data interface{}) (string, error) {
		return "NotImplemented", nil
	}

	// Add the 'tpl' function here
	funcMap["tpl"] = func(tpl string, vals map[string]interface{}) (string, error) {
		clone, err := t.Clone()
		if err != nil {
			return "", errors.Errorf("clone template failed: %v", err)
		}

		result, err := e.renderWithTemplate(tpl, clone)
		if err != nil {
			return "", errors.Wrapf(err, "error during tpl function execution for %q", tpl)
		}
		return result.String(), nil
	}

	// Add the `required` function here so we can use lintMode
	funcMap["required"] = func(warn string, val interface{}) (interface{}, error) {
		if val == nil {
			return val, errors.Errorf(warnWrap(warn))
		} else if _, ok := val.(string); ok {
			if val == "" {
				return val, errors.Errorf(warnWrap(warn))
			}
		}
		return val, nil
	}

	t.Funcs(funcMap)
}

// renderWithTemplate takes a map of templates/values to render using
// passed Template object.
func (e Engine) renderWithTemplate(tmpl string, t *template.Template) (out *bytes.Buffer, err error) {
	// Basically, what we do here is start with an empty parent template and then
	// build up a list of templates -- one for each file. Once all of the templates
	// have been parsed, we loop through again and execute every template.
	//
	// The idea with this process is to make it possible for more complex templates
	// to share common blocks, but to make the entire thing feel like a file-based
	// template engine.
	//
	// Template from tpl function is a dublicate, so defines in tpl are not interfered
	// with defines in "real" templates.
	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("rendering template failed: %v", r)
		}
	}()

	e.initFunMap(t)

	_, err = t.New(e.Name).Parse(tmpl)
	if err != nil {
		return nil, cleanupParseError(e.Name, err)
	}

	data := e.deepCopyData()
	data["Files"] = Files{}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, e.Name, data); err != nil {
		return nil, cleanupExecError(e.Name, err)
	}

	return &buf, nil
}

func cleanupParseError(filename string, err error) error {
	tokens := strings.Split(err.Error(), ": ")
	if len(tokens) == 1 {
		// This might happen if a non-templating error occurs
		return fmt.Errorf("parse error in (%s): %s", filename, err)
	}
	// The first token is "template"
	// The second token is either "filename:lineno" or "filename:lineNo:columnNo"
	location := tokens[1]
	// The remaining tokens make up a stacktrace-like chain, ending with the relevant error
	errMsg := tokens[len(tokens)-1]
	return fmt.Errorf("parse error at (%s): %s", location, errMsg)
}

func cleanupExecError(filename string, err error) error {
	if _, isExecError := err.(template.ExecError); !isExecError {
		return err
	}

	tokens := strings.SplitN(err.Error(), ": ", 3)
	if len(tokens) != 3 {
		// This might happen if a non-templating error occurs
		return fmt.Errorf("execution error in (%s): %s", filename, err)
	}

	// The first token is "template"
	// The second token is either "filename:lineno" or "filename:lineNo:columnNo"
	location := tokens[1]

	parts := warnRegex.FindStringSubmatch(tokens[2])
	if len(parts) >= 2 {
		return fmt.Errorf("execution error at (%s): %s", location, parts[1])
	}

	return err
}

const (
	warnStartDelim = "HELM_ERR_START"
	warnEndDelim   = "HELM_ERR_END"
)

var warnRegex = regexp.MustCompile(warnStartDelim + `(.*)` + warnEndDelim)

func warnWrap(warn string) string {
	return warnStartDelim + warn + warnEndDelim
}

// mocking Helm's .Files.Get
type Files struct {
}

// implements .Files.Get
// helm version of .Files.Get returns empty string if file does not exists
// https://github.com/helm/helm/blob/main/pkg/engine/files.go#L42-L54
func (_ Files) Get(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(contents), nil
}
