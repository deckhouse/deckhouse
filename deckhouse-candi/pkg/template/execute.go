package template

import (
	"bytes"
	"flant/deckhouse-candi/pkg/log"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const (
	tmpDirPrefix = "candi-bundle-"

	bundlePermissions = 0700
)

type RenderedTemplate struct {
	Content  *bytes.Buffer
	FileName string
}

func formatDir(dir string) string {
	return strings.TrimSuffix(dir, "/") + "/"
}

// RenderTemplate renders each file in templatesDir.
// Files are rendered separately, so no support for
// libraries, like in Helm.
func RenderTemplate(templatesDir string, data map[string]interface{}) ([]RenderedTemplate, error) {
	templatesDir = formatDir(templatesDir)

	files, err := ioutil.ReadDir(templatesDir)
	if os.IsNotExist(err) {
		log.InfoF("Templates directory %q does not exist. Skipping...\n", templatesDir)
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read templates dir: %v", err)
	}

	renderedTemplates := make([]RenderedTemplate, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		fileName := file.Name()
		if !strings.HasSuffix(fileName, ".tpl") {
			continue
		}

		templatePath := templatesDir + file.Name()

		// Render template as a chart with one template to use helm functions.
		tmplData, err := ioutil.ReadFile(templatePath)
		if err != nil {
			return nil, fmt.Errorf("read template file '%s': %v", templatePath, err)
		}

		// render chart with prepared values
		var e Engine
		e.Name = file.Name()
		e.Data = data
		out, err := e.Render(tmplData)
		if err != nil {
			return nil, fmt.Errorf("render template file '%s': %v", templatePath, err)
		}

		renderedTemplates = append(renderedTemplates, RenderedTemplate{
			Content:  out,
			FileName: strings.TrimSuffix(filepath.Base(templatePath), ".tpl"),
		})
	}

	return renderedTemplates, nil
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
	tmpDir = strings.TrimSuffix(tmpDir, "/")
	_ = os.Mkdir(tmpDir, bundlePermissions)

	tmpDir, err = ioutil.TempDir(tmpDir, tmpDirPrefix)
	if err != nil {
		panic(err)
	}
	return &Controller{TmpDir: tmpDir}
}

func (t *Controller) RenderAndSaveTemplates(fromDir, toDir string, data map[string]interface{}) error {
	templates, err := RenderTemplate(fromDir, data)
	if err != nil {
		return fmt.Errorf("render templates: %v", err)
	}

	err = SaveTemplatesToDir(templates, t.TmpDir+toDir)
	if err != nil {
		return fmt.Errorf("save templates: %v", err)
	}

	return nil
}

func (t *Controller) RenderBashBooster(fromDir, toDir string) error {
	bashBooster, err := RenderBashBooster(fromDir)
	if err != nil {
		return fmt.Errorf("render bashboster: %v", err)
	}

	err = ioutil.WriteFile(t.TmpDir+toDir+"/bashbooster.sh", []byte(bashBooster), bundlePermissions)
	if err != nil {
		return fmt.Errorf("save bashboster: %v", err)
	}
	return nil
}

func (t *Controller) Close() {
	_ = os.RemoveAll(t.TmpDir)
}
