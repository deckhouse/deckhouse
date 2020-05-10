package linter

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/modules"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/types"
)

//
func Run(tmpDir string, m types.Module) error {
	// Silence default logger (helm)
	log.SetOutput(ioutil.Discard)

	f, err := LoadConfiguration(m.Path+"/"+modules.ValuesConfigFilename, "", tmpDir)
	if err != nil {
		return fmt.Errorf("configuration loading error: %v", err)
	}
	defer f.Close()

	f.FindAll()

	values, err := f.ReturnValues()
	if err != nil {
		return fmt.Errorf("saving values error: %v", err)
	}

	return NewModuleController(m, values).Run()
}
