package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/romana/rlog"
)

// InitHelm запускает установку tiller-a.
func InitHelm() {
	rlog.Info("HELM-INIT run helm init")

	stdout, stderr, err := HelmCmd("init", "--service-account", "antiopa", "--upgrade", "--wait", "--skip-refresh")
	if err != nil {
		rlog.Errorf("HELM-INIT: %s\n%s %s", err, stdout, stderr)
		os.Exit(1)
	}
	rlog.Infof("HELM-INIT Tiller initialization done: %v %v", stdout, stderr)

	stdout, stderr, err = HelmCmd("version")
	if err != nil {
		rlog.Errorf("HELM-INIT Unable to get helm version: %v\n%v %v", err, stdout, stderr)
		os.Exit(1)
	}
	rlog.Infof("HELM-INIT helm version:\n%v %v", stdout, stderr)

	rlog.Info("HELM-INIT Successfully initialized")
}

// HelmTillerNamespace возвращает имя namespace, куда устаналивается tiller
// Можно ставить в другой namespace, можно в тот же, где сама antiopa.
// TODO Есть переменная TILLER_NAMESPACE - можно её поставить ещё на этапе деплоя
func HelmTillerNamespace() string {
	return KubernetesAntiopaNamespace
	//return fmt.Sprintf("%s-tiller", KubernetesAntiopaNamespace)
}

// HelmCmd запускает helm с переданными аргументами
// Перед запуском устанавливает переменную среды TILLER_NAMESPACE
// чтобы antiopa работала со своим tiller-ом
func HelmCmd(args ...string) (stdout string, stderr string, err error) {
	cmd := exec.Command("/usr/local/bin/helm", args...)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("TILLER_NAMESPACE=%s", HelmTillerNamespace()),
	)
	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdout = strings.TrimSpace(stdoutBuf.String())
	stderr = strings.TrimSpace(stderrBuf.String())

	return
}

func HelmDeleteSingleFailedRevision(releaseName string) (err error) {
	revision, status, err := HelmLastReleaseStatus(releaseName)
	if err != nil {
		if revision != "0" {
			rlog.Infof("%v", err)
		}
		return err
	}

	//  No interest of revisions older than 1
	if revision == "1" && status == "FAILED" {
		// delete and purge!
		err = HelmDelete(releaseName)
		if err != nil {
			rlog.Infof("Error deleting first failed release '%s': %v", releaseName, err)
			return err
		}
		rlog.Infof("  Single failed release for '%s' deleted", releaseName)
	} else {
		rlog.Debugf("Release '%s' has revision '%s' with status %s", releaseName, revision, status)
	}

	return
}

// Get last known revision and status
// helm history output:
// REVISION	UPDATED                 	STATUS    	CHART                 	DESCRIPTION
// 1        Fri Jul 14 18:25:00 2017	SUPERSEDED	symfony-demo-0.1.0    	Install complete
func HelmLastReleaseStatus(releaseName string) (revision string, status string, err error) {
	stdout, stderr, err := HelmCmd("history", releaseName)
	if err != nil {
		err = fmt.Errorf("Cannot get history for release '%s'\n%v %v", releaseName, stdout, stderr)
		return
	}
	historyLines := strings.Split(stdout, "\n")
	firstLine := historyLines[0]
	if strings.Contains(firstLine, "Error:") && strings.Contains(firstLine, "not found") {
		// Bad module name or no releases installed
		err = fmt.Errorf("No release '%s' found\n%v %v", releaseName, stdout, stderr)
		revision = "0"
		return
	}
	lastLine := historyLines[len(historyLines)-1]
	fields := regexp.MustCompile("\\t").Split(lastLine, 5)
	revision = strings.TrimSpace(fields[0])
	status = strings.TrimSpace(fields[2])
	return
}

func HelmDelete(releaseName string) (err error) {
	stdout, stderr, err := HelmCmd("delete", "--purge", releaseName)
	if err != nil {
		return fmt.Errorf("helm delete --purge %s invocation error: %v\n%v %v", releaseName, err, stdout, stderr)
	}
	return
}
