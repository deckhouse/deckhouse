package template

import (
	"fmt"
	"io/ioutil"
	"os"
)

func SaveTemplatesToDir(templates []RenderedTemplate, dirToSave string) error {
	if err := os.MkdirAll(dirToSave, os.ModePerm); err != nil {
		return fmt.Errorf("creating templates dir: %v", err)
	}

	for _, tpl := range templates {
		err := ioutil.WriteFile(formatDir(dirToSave)+tpl.FileName, tpl.Content.Bytes(), bundlePermissions)
		if err != nil {
			return fmt.Errorf("saving template file %s: %v", tpl.FileName, err)
		}
	}
	return nil
}
