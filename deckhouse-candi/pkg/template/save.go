package template

import (
	"fmt"
	"io/ioutil"
	"os"
)

func SaveTemplatesToDir(templates []RenderedTemplate, dirToSave string) error {
	err := os.MkdirAll(dirToSave, os.ModePerm)
	if err != nil {
		return err
	}

	for _, tpl := range templates {
		if err := ioutil.WriteFile(formatDir(dirToSave)+tpl.FileName, tpl.Content.Bytes(), 0755); err != nil {
			return fmt.Errorf("saving file %s: %v", tpl.FileName, err)
		}
	}
	return nil
}
