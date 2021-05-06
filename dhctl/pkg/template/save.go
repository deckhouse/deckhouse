package template

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

func SaveRenderedToDir(renderedTpls []RenderedTemplate, dirToSave string) error {
	if err := os.MkdirAll(dirToSave, os.ModePerm); err != nil {
		return fmt.Errorf("creating rendered templates dir: %v", err)
	}

	for _, rendered := range renderedTpls {
		filename := path.Join(dirToSave, rendered.FileName)
		err := ioutil.WriteFile(filename, rendered.Content.Bytes(), bundlePermissions)
		if err != nil {
			return fmt.Errorf("saving rendered file %s: %v", rendered.FileName, err)
		}
	}

	return nil
}
