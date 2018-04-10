package helm

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/romana/rlog"
)

type HelmClient interface {
	TillerNamespace() string
	CommandEnv() []string
	Cmd(args ...string) (string, string, error)
	DeleteSingleFailedRevision(releaseName string) error
	LastReleaseStatus(releaseName string) (string, string, error)
	DeleteRelease(releaseName string) error
}

type CliHelm struct {
	tillerNamespace string
}

// InitHelm запускает установку tiller-a.
func Init(tillerNamespace string) (HelmClient, error) {
	rlog.Info("HELM-INIT run helm init")

	helm := &CliHelm{tillerNamespace: tillerNamespace}

	stdout, stderr, err := helm.Cmd("init", "--service-account", "antiopa", "--upgrade", "--wait", "--skip-refresh")
	if err != nil {
		return nil, fmt.Errorf("%s\n%s %s", err, stdout, stderr)
	}
	rlog.Infof("HELM-INIT Tiller initialization done: %v %v", stdout, stderr)

	stdout, stderr, err = helm.Cmd("version")
	if err != nil {
		return nil, fmt.Errorf("unable to get helm version: %v\n%v %v", err, stdout, stderr)
	}
	rlog.Infof("HELM-INIT helm version:\n%v %v", stdout, stderr)

	rlog.Info("HELM-INIT Successfully initialized")

	return helm, nil
}

func (helm *CliHelm) TillerNamespace() string {
	return helm.tillerNamespace
}

func (helm *CliHelm) CommandEnv() []string {
	res := make([]string, 0)
	res = append(res, fmt.Sprintf("TILLER_NAMESPACE=%s", helm.TillerNamespace()))
	return res
}

// Запускает helm с переданными аргументами.
// Перед запуском устанавливает переменную среды TILLER_NAMESPACE,
// чтобы antiopa работала со своим tiller-ом.
func (helm *CliHelm) Cmd(args ...string) (stdout string, stderr string, err error) {
	cmd := exec.Command("/usr/local/bin/helm", args...)
	cmd.Env = append(os.Environ(), helm.CommandEnv()...)

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdout = strings.TrimSpace(stdoutBuf.String())
	stderr = strings.TrimSpace(stderrBuf.String())

	return
}

func (helm *CliHelm) DeleteSingleFailedRevision(releaseName string) (err error) {
	revision, status, err := helm.LastReleaseStatus(releaseName)
	if err != nil {
		if revision != "0" {
			rlog.Infof("%v", err)
		}
		return err
	}

	//  No interest of revisions older than 1
	if revision == "1" && status == "FAILED" {
		// delete and purge!
		err = helm.DeleteRelease(releaseName)
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
func (helm *CliHelm) LastReleaseStatus(releaseName string) (revision string, status string, err error) {
	stdout, stderr, err := helm.Cmd("history", releaseName)
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

func (helm *CliHelm) DeleteRelease(releaseName string) (err error) {
	stdout, stderr, err := helm.Cmd("delete", "--purge", releaseName)
	if err != nil {
		return fmt.Errorf("helm delete --purge %s invocation error: %v\n%v %v", releaseName, err, stdout, stderr)
	}
	return
}
