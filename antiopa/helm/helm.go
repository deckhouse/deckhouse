package helm

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/romana/rlog"
)

var (
	TillerNamespace string
)

// InitHelm запускает установку tiller-a.
func Init(tillerNamespace string) {
	TillerNamespace = tillerNamespace

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

func CommandEnv() []string {
	res := make([]string, 0)
	res = append(res, fmt.Sprintf("TILLER_NAMESPACE=%s", TillerNamespace))
	return res
}

// HelmCmd запускает helm с переданными аргументами
// Перед запуском устанавливает переменную среды TILLER_NAMESPACE
// чтобы antiopa работала со своим tiller-ом
func HelmCmd(args ...string) (stdout string, stderr string, err error) {
	cmd := exec.Command("/usr/local/bin/helm", args...)
	cmd.Env = append(os.Environ(), CommandEnv()...)

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdout = strings.TrimSpace(stdoutBuf.String())
	stderr = strings.TrimSpace(stderrBuf.String())

	return
}
