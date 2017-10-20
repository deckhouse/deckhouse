package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/romana/rlog"
)

// HelmInit запускает установку tiller-a.
func HelmInit() {
	rlog.Infof("Start helm init. Use TILLER_NAMESPACE=%s", HelmTillerNamespace())

	stdout, stderr, err := HelmCmd("init")

	if err != nil {
		rlog.Errorf("helm init error: %v", err)
	}

	rlog.Infof("helm init output: %v %v", stdout, stderr)

	stdout, stderr, err = HelmCmd("version")

	if err != nil {
		rlog.Errorf("helm version error: %v", err)
	}

	rlog.Infof("helm version output: %v %v", stdout, stderr)
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
