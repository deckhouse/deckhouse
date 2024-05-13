/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pkg

import (
	"bytes"
	"os"
	"text/template"
)

func RenderTemplateFiles(filePath string, data interface{}) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		panic(err)
	}

	strContent, err := RenderTemplate(string(content), data)
	if err != nil {
		return err
	}

	err = OsWriteFile(filePath, []byte(strContent), fileInfo.Mode())
	return err
}

func RenderTemplate(templateContent string, data interface{}) (string, error) {
	tmpl, err := template.New("").Parse(templateContent)
	if err != nil {
		return "", err
	}

	var resultBuffer bytes.Buffer

	err = tmpl.Execute(&resultBuffer, data)
	if err != nil {
		return "", err
	}

	return resultBuffer.String(), nil
}
