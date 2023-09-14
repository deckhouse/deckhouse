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

package helper

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const defaultPerm = 0777

// DeckhouseRoot get deckhouse root dirrectory.
func DeckhouseRoot() (path string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	if filepath.Base(cwd) != "tools" {
		return "", errors.New("wrong directory. Run tools from .. deckhouse/tools/ directory")
	}

	return filepath.Dir(cwd), err
}

// NewRenderDir create a new temporary directory following the default helm template.
func NewRenderDir(chartName string) (path string, err error) {
	renderdir, err := os.MkdirTemp("", "renderdir")
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Join(renderdir, "/charts"), defaultPerm); err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Join(renderdir, "/templates"), defaultPerm); err != nil {
		return "", err
	}

	chartData := fmt.Sprintf("name: %s\nversion: 0.0.1", chartName)
	if err := os.WriteFile(filepath.Join(renderdir, "Chart.yaml"), []byte(chartData), defaultPerm); err != nil {
		return "", err
	}

	deckhouseRoot, err := DeckhouseRoot()
	if err != nil {
		return "", err
	}
	helmLibPath := "helm_lib/charts/deckhouse_lib_helm"

	if err := os.Symlink(filepath.Join(deckhouseRoot, helmLibPath), filepath.Join(renderdir, "/charts/helm_lib")); err != nil {
		return "", err
	}

	return renderdir, nil
}
