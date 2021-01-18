package operations

import (
	"io/ioutil"
	"os"
	"os/exec"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/candictl/pkg/app"
	"github.com/deckhouse/deckhouse/candictl/pkg/config"
	"github.com/deckhouse/deckhouse/candictl/pkg/log"
)

func Edit(data []byte) ([]byte, error) {
	schemaStore := config.NewSchemaStore()

	editor := app.Editor
	if editor == "" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
	}

	tmpFile, err := ioutil.TempFile(app.TmpDirName, "candictl-editor.*.yaml")
	if err != nil {
		log.ErrorF("can't save cluster configuration: %s\n", err)
		return nil, err
	}

	err = ioutil.WriteFile(tmpFile.Name(), data, 0600)
	if err != nil {
		log.ErrorF("can't write write cluster configuration to the file %s: %s\n", tmpFile.Name(), err)
		return nil, err
	}

	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return nil, err
	}

	modifiedData, err := ioutil.ReadFile(tmpFile.Name())
	if err != nil {
		return nil, err
	}

	_, err = schemaStore.Validate(&modifiedData)
	if err != nil {
		return nil, err
	}

	modifiedData, err = yaml.JSONToYAML(modifiedData)
	if err != nil {
		return nil, err
	}

	return modifiedData, nil
}
