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

package template

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const (
	tmpDirPrefix = "candi-bundle-"

	bundlePermissions = 0o700
)

type RenderedTemplate struct {
	Content  *bytes.Buffer
	FileName string
}

// RenderTemplatesDir renders each file in templatesDir.
// Files are rendered separately, so no support for
// libraries, like in Helm.
func RenderTemplatesDir(templatesDir string, data map[string]interface{}, ignoreMap map[string]struct{}) ([]RenderedTemplate, error) {
	files, err := os.ReadDir(templatesDir)
	if os.IsNotExist(err) {
		log.InfoF("Templates directory %q does not exist. Skipping...\n", templatesDir)
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read templates dir: %v", err)
	}

	renders := make([]RenderedTemplate, 0, len(files))

	for _, file := range files {
		if _, ok := ignoreMap[filepath.Join(templatesDir, file.Name())]; ok {
			continue
		}

		tplName := file.Name()

		isTemplate := !file.IsDir() && strings.HasSuffix(tplName, ".tpl")
		if !isTemplate {
			continue
		}

		tplPath := filepath.Join(templatesDir, tplName)

		// Render template as a chart with one template to use helm functions.
		tplContent, err := os.ReadFile(tplPath)
		if err != nil {
			return nil, fmt.Errorf("read template file '%s': %v", tplPath, err)
		}

		rendered, err := RenderTemplate(tplName, tplContent, data)
		if err != nil {
			return nil, fmt.Errorf("render template file '%s': %v", tplPath, err)
		}
		renders = append(renders, *rendered)
	}

	return renders, nil
}

func RenderTemplate(name string, content []byte, data map[string]interface{}) (*RenderedTemplate, error) {
	// render chart with prepared values
	e := Engine{
		Name: name,
		Data: data,
	}

	out, err := e.Render(content)
	if err != nil {
		return nil, err
	}

	rendered := &RenderedTemplate{
		Content:  out,
		FileName: strings.TrimSuffix(name, ".tpl"),
	}

	return rendered, nil
}

type Controller struct {
	TmpDir string
}

func NewTemplateController(tmpDir string) *Controller {
	var err error
	if tmpDir == "" {
		tmpDir = os.TempDir()
	} else {
		tmpDir, err = filepath.Abs(tmpDir)
		if err != nil {
			panic(err)
		}
	}
	_ = os.Mkdir(tmpDir, bundlePermissions)

	tmpDir, err = os.MkdirTemp(tmpDir, tmpDirPrefix)
	if err != nil {
		panic(err)
	}
	return &Controller{TmpDir: tmpDir}
}

func (t *Controller) RenderAndSaveTemplates(fromDir, toDir string, data map[string]interface{}, ignoreMap map[string]struct{}) error {
	renderedTemplates, err := RenderTemplatesDir(fromDir, data, ignoreMap)
	if err != nil {
		return fmt.Errorf("render templates: %v", err)
	}

	err = SaveRenderedToDir(renderedTemplates, filepath.Join(t.TmpDir, toDir))
	if err != nil {
		return fmt.Errorf("save rendered templates: %v", err)
	}

	return nil
}

func (t *Controller) RenderBashBooster(fromDir, toDir string, data map[string]interface{}) error {
	bashBooster, err := RenderBashBooster(fromDir, data)
	if err != nil {
		return fmt.Errorf("render bashboster: %v", err)
	}

	filename := filepath.Join(t.TmpDir, toDir, "bashbooster.sh")
	err = os.WriteFile(filename, []byte(bashBooster), bundlePermissions)
	if err != nil {
		return fmt.Errorf("save bashboster: %v", err)
	}
	return nil
}

func (t *Controller) Close() {
	_ = os.RemoveAll(t.TmpDir)
}
