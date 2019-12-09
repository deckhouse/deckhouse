package runner

import (
	"fmt"

	"github.com/deckhouse/deckhouse/testing/matrix/values"
)

func RunLint(moduleDir, tmpDir string) error {
	f, err := values.LoadConfiguration(moduleDir+"/"+ValuesConfigFilename, "", tmpDir)
	if err != nil {
		return fmt.Errorf("configuration loading error: %v", err)
	}
	defer f.Close()

	f.FindAll()
	err = f.SaveValues()
	if err != nil {
		return fmt.Errorf("saving values error: %v", err)
	}

	c, err := NewModuleController(f.TmpDir, moduleDir)
	if err != nil {
		return fmt.Errorf("creating new module controller for %q failed: %v", moduleDir, err)
	}
	err = c.Run()
	if err != nil {
		return err
	}
	return nil
}
