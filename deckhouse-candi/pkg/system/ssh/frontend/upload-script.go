package frontend

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/system/ssh/session"
)

type UploadScript struct {
	Session *session.Session

	ScriptPath string
	Args       []string

	sudo bool

	stdoutHandler func(string)
}

func NewUploadScript(sess *session.Session, scriptPath string, args ...string) *UploadScript {
	return &UploadScript{
		Session:    sess,
		ScriptPath: scriptPath,
		Args:       args,
	}
}

func (u *UploadScript) Sudo() *UploadScript {
	u.sudo = true
	return u
}

func (u *UploadScript) WithStdoutHandler(handler func(string)) *UploadScript {
	u.stdoutHandler = handler
	return u
}

func (u *UploadScript) Execute() (stdout []byte, err error) {
	scriptName := filepath.Base(u.ScriptPath)

	remotePath := "."
	if u.sudo {
		remotePath = "/tmp/" + scriptName
	}
	err = NewFile(u.Session).Upload(u.ScriptPath, remotePath)
	if err != nil {
		return nil, fmt.Errorf("upload: %v", err)
	}

	var cmd *Command
	if u.sudo {
		cmd = NewCommand(u.Session, "/tmp/"+scriptName, u.Args...).Sudo()
	} else {
		cmd = NewCommand(u.Session, "./"+scriptName, u.Args...).Cmd()
	}

	scriptCmd := cmd.CaptureStdout(nil)
	if u.stdoutHandler != nil {
		scriptCmd = scriptCmd.WithStdoutHandler(u.stdoutHandler)
	}

	err = scriptCmd.Run()
	if err != nil {
		err = fmt.Errorf("execute on remote: %v", err)
	}
	return cmd.StdoutBytes(), err
}

func (u *UploadScript) ExecuteBundle(parentDir string, bundleDir string) (stdout []byte, err error) {
	var bundleName = fmt.Sprintf("bundle-%s.tar", time.Now().Format("20060102-150405"))

	// tar cpf bundle.tar -C /tmp/deckhouse-candi.1231qd23/var/lib bashible
	tarCmd := exec.Command("tar", "cpf", bundleName, "-C", parentDir, bundleDir)
	err = tarCmd.Run()
	if err != nil {
		return nil, fmt.Errorf("tar bundle: %v", err)
	}

	// upload to /tmp
	err = NewFile(u.Session).Upload(bundleName, "/tmp")
	if err != nil {
		return nil, fmt.Errorf("upload: %v", err)
	}

	// sudo:
	// tar xpof /tmp/bundle.tar -C /var/lib && /var/lib/bashible/bashible.sh args...
	tarCmdline := fmt.Sprintf("tar xpof /tmp/%s -C /var/lib && /var/lib/%s/%s %s", bundleName, bundleDir, u.ScriptPath, strings.Join(u.Args, " "))
	bundleCmd := NewCommand(u.Session, tarCmdline).Sudo()

	// Buffers to implement output handler logic
	lastStep := ""

	err = bundleCmd.WithStdoutHandler(bundleOutputHandler(&lastStep)).CaptureStdout(nil).Run()
	if err != nil {
		if lastStep != "" {
			logboek.LogProcessFail(log.BoldFailOptions())
		}
		err = fmt.Errorf("execute bundle: %v", err)
	} else {
		logboek.LogProcessEnd(log.BoldEndOptions())
	}
	return bundleCmd.StdoutBytes(), err
}

var stepHeaderRegexp = regexp.MustCompile("^=== Step: /var/lib/bashible/bundle_steps/(.*)$")

func bundleOutputHandler(lastStep *string) func(string) {
	return func(l string) {
		if l == "===" {
			return
		}
		if stepHeaderRegexp.Match([]byte(l)) {
			match := stepHeaderRegexp.FindStringSubmatch(l)
			stepName := match[1]

			if *lastStep == stepName {
				logboek.LogProcessFail(log.BoldFailOptions())
				stepName = "Retry " + stepName
			} else if *lastStep != "" {
				logboek.LogProcessEnd(log.BoldEndOptions())
			}

			logboek.LogProcessStart("Run step "+stepName, log.BoldStartOptions())
			*lastStep = match[1]
			return
		}
		logboek.LogInfoLn(l)
	}
}
