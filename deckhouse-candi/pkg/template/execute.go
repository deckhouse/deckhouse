package template

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/helm/helm/pkg/engine"
)

const tmpDirPrefix = "candi-"

func prepareFuncMap() template.FuncMap {
	funcMap := engine.FuncMap()
	funcMap["toYAML"] = funcMap["toYaml"]
	funcMap["has"] = funcMap["hasKey"]

	return funcMap
}

type RenderedTemplate struct {
	Content  *bytes.Buffer
	FileName string
}

func formatDir(dir string) string {
	return strings.TrimSuffix(dir, "/") + "/"
}

func RenderTemplate(templatesDir string, data interface{}) ([]RenderedTemplate, error) {
	templatesDir = formatDir(templatesDir)

	files, err := ioutil.ReadDir(templatesDir)
	if os.IsNotExist(err) {
		// TODO: Fix this
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read dir: %v", err)
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
		tmpls, err := template.New(file.Name()).Funcs(prepareFuncMap()).ParseFiles(templatePath)
		if err != nil {
			return nil, fmt.Errorf("parse files: %v", err)
		}

		buff := new(bytes.Buffer)
		err = tmpls.Execute(buff, &data)
		if err != nil {
			return nil, err
		}
		renderedTemplates = append(renderedTemplates, RenderedTemplate{
			Content: buff, FileName: strings.TrimSuffix(filepath.Base(templatePath), ".tpl"),
		})
	}

	return renderedTemplates, nil
}

type TemplateController struct {
	TmpDir string
}

func NewTemplateController(tmpDir string) *TemplateController {
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
	_ = os.Mkdir(tmpDir, 0755)

	tmpDir, err = ioutil.TempDir(tmpDir, tmpDirPrefix)
	if err != nil {
		panic(err)
	}
	return &TemplateController{TmpDir: tmpDir}
}

func (t *TemplateController) RenderAndSaveTemplates(fromDir, toDir string, data interface{}) error {
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

func (t *TemplateController) RenderBashBooster(fromDir, toDir string) error {
	bashBooster, err := RenderBashBooster(fromDir)
	if err != nil {
		return fmt.Errorf("render bashboster: %v", err)
	}

	err = ioutil.WriteFile(t.TmpDir+toDir+"/bashbooster.sh", []byte(bashBooster), 0700)
	if err != nil {
		return fmt.Errorf("save bashboster: %v", err)
	}
	return nil
}

func (t *TemplateController) Close() {
	_ = os.RemoveAll(t.TmpDir)
}
