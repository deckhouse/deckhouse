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

package alertimagechecks

import (
	"io"
	"os"
	"path/filepath"
	"tools/helm_generate/helper"

	"github.com/deckhouse/deckhouse/testing/library/helm"
)

func run() error {
	renderContent, err := renderHelmTemplate("340-extended-monitoring/monitoring/prometheus-rules/image-availability/image-checks.tpl")
	if err != nil {
		return err
	}

	io.WriteString(os.Stdout, renderContent["extended-monitoring/templates/image-checks"])
	return nil
}

func renderHelmTemplate(template string) (map[string]string, error) {
	deckhouseRoot, err := helper.DeckhouseRoot()
	if err != nil {
		return nil, err
	}
	renderDirPath, err := helper.NewRenderDir("extended-monitoring")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(renderDirPath)

	templateFullPath := filepath.Join(filepath.Join(deckhouseRoot, "modules", template))
	if err := os.Symlink(templateFullPath, filepath.Join(renderDirPath, "/templates/image-checks")); err != nil {
		return nil, err
	}

	r := helm.Renderer{}
	resp, err := r.RenderChartFromDir(renderDirPath, "{}")

	return resp, err
}
