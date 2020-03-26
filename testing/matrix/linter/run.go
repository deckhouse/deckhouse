package linter

import (
	"fmt"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/modules"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/types"
)

//
func Run(tmpDir string, m types.Module) error {
	f, err := LoadConfiguration(m.Path+"/"+modules.ValuesConfigFilename, "", tmpDir)
	if err != nil {
		return fmt.Errorf("configuration loading error: %v", err)
	}
	defer f.Close()

	f.FindAll()

	err = f.SaveValues()
	if err != nil {
		return fmt.Errorf("saving values error: %v", err)
	}

	c := NewModuleController(f.TmpDir, m)

	err = c.Run()
	if err != nil {
		return err
	}

	return nil
}
