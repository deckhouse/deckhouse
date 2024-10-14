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

package linter

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/modules"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/utils"
)

// applyDigests if ugly because values now are strongly untyped. We have to rewrite this after adding proper global schema
func applyDigests(digests map[string]interface{}, values interface{}) {
	values.(map[string]interface{})["global"].(map[string]interface{})["modulesImages"].(map[string]interface{})["digests"] = digests
}

func isExist(baseDir, filename string) bool {
	_, err := os.Stat(filepath.Join(baseDir, filename))
	return err == nil
}

func Run(tmpDir string, m utils.Module) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic on linter run occurred:\n%s\n\n%v", r, string(debug.Stack()))
		}
	}()

	// Silence default loggers
	log.SetOutput(io.Discard)          // helm
	logrus.SetLevel(logrus.PanicLevel) // shell-operator

	var values []chartutil.Values
	if err != nil {
		return err
	}

	if isExist(m.Path, filepath.Join("monitoring", "prometheus-rules")) && !modules.PromtoolAvailable() {
		return errors.New("promtool is not available, execute `make bin/promtool` prior to starting matrix tests")
	}

	if isExist(m.Path, "openapi") && !isExist(m.Path, "values_matrix_test.yaml") {
		values, err = ComposeValuesFromSchemas(m)
		if err != nil {
			return fmt.Errorf("saving values from openapi: %v", err)
		}
	} else {
		f, err := LoadConfiguration(m, filepath.Join(m.Path, modules.ValuesConfigFilename), "", tmpDir)
		if err != nil {
			return fmt.Errorf("configuration loading error: %v", err)
		}
		defer f.Close()

		f.FindAll()

		values, err = f.ReturnValues()
		if err != nil {
			return fmt.Errorf("saving values error: %v", err)
		}
	}

	err = NewModuleController(m, values).Run()
	return
}
