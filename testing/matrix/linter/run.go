package linter

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/modules"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/utils"
)

// applyTags if ugly because values now are strongly untyped. We have to rewrite this after adding proper global schema
func applyTags(tags map[string]map[string]string, values interface{}) {
	values.(map[string]interface{})["global"].(map[string]interface{})["modulesImages"].(map[string]interface{})["tags"] = tags
}

func isExist(baseDir, filename string) bool {
	_, err := os.Stat(filepath.Join(baseDir, filename))
	return err == nil
}

func Run(tmpDir string, m utils.Module) error {
	// Silence default loggers
	log.SetOutput(ioutil.Discard)      // helm
	logrus.SetLevel(logrus.PanicLevel) // shell-operator

	var values []string
	var err error
	if isExist(m.Path, "openapi") && !isExist(m.Path, "values_matrix_test.yaml") {
		values, err = ComposeValuesFromSchemas(m)
		if err != nil {
			return fmt.Errorf("saving values from openapi: %v", err)
		}
	} else {
		f, err := LoadConfiguration(filepath.Join(m.Path, modules.ValuesConfigFilename), "", tmpDir)
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

	return NewModuleController(m, values).Run()
}
