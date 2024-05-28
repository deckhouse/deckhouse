package preflight

import (
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

func (pc *Checker) CheckMasterHostname() error {
	if app.PreflightSkipMasterHostname {
		log.InfoLn("Master hostname preflight check was skipped")
		return nil
	}

	if pc.sshClient.Settings.CountHosts() < 2 {
		log.DebugLn("Master hostname preflight check was skipped")
		return nil
	}
	log.DebugLn("Checking if localhost domain resolves correctly")

	file, err := template.RenderAndSavePreflightCheckScript("get_hostname.sh", nil)
	if err != nil {
		return err
	}

	masterHostnames := make(map[string]struct{})
	masterWithError := make(map[string]string)

	for range pc.sshClient.Settings.AvailableHosts() {
		scriptCmd := pc.sshClient.UploadScript(file)
		out, err := scriptCmd.Execute()
		if err != nil {
			log.ErrorLn(strings.Trim(string(out), "\n"))
			return fmt.Errorf(
				"could not execute a script to check master hostname: %w",
				err,
			)
		}
		if _, ok := masterHostnames[string(out)]; ok {
			masterWithError[pc.sshClient.Settings.Host()] = string(out)
			pc.sshClient.Settings.ChoiceNewHost()
			continue
		}

		masterHostnames[string(out)] = struct{}{}
		pc.sshClient.Settings.ChoiceNewHost()
	}

	return nil
}
