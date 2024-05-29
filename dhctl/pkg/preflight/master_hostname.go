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
		log.DebugF("Get hostname from master %s\n", pc.sshClient.Settings.Host())
		scriptCmd := pc.sshClient.UploadScript(file)
		out, err := scriptCmd.Execute()
		if err != nil {
			log.ErrorLn(strings.Trim(string(out), "\n"))
			return fmt.Errorf(
				"could not execute a script to get master hostname: %w",
				err,
			)
		}
		hostname := string(out)
		log.DebugF("Master: %s hostname: %s\n", pc.sshClient.Settings.Host(), hostname)
		if _, ok := masterHostnames[hostname]; ok {
			log.ErrorF("Master with hostname %s already exist!\n", hostname)
			masterWithError[pc.sshClient.Settings.Host()] = hostname
			pc.sshClient.Settings.ChoiceNewHost()
			continue
		}

		masterHostnames[hostname] = struct{}{}
		pc.sshClient.Settings.ChoiceNewHost()
	}

	if len(masterWithError) > 0 {
		servers := []string{}
		for k := range masterWithError {
			servers = append(servers, k)
		}
		return fmt.Errorf(
			"please set unique hostname on the servers %s and re-install the installation again",
			strings.Join(servers, ","),
		)
	}

	return nil
}
