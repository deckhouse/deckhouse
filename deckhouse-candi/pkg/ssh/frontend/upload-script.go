package frontend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"flant/deckhouse-candi/pkg/ssh/session"
)

type UploadScript struct {
	Session *session.Session

	ScriptPath string
	Args       []string

	sudo bool
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

	err = cmd.EnableLive().
		CaptureStdout(nil).
		Run()
	if err != nil {
		err = fmt.Errorf("execute on remote: %v", err)
	}
	return cmd.StdoutBytes(), err
}

func (u *UploadScript) ExecuteBundle(parentDir string, bundleDir string) (stdout []byte, err error) {
	var bundleName = fmt.Sprintf("bundle-%s.tar", time.Now().Format("20060102-150405"))

	// tar cpf bundle.tar -C /tmp/deckhouse-candi.1231qd23/var/lib bashible
	tarCmd := exec.Command("tar", "cpf", bundleName, "-C", parentDir, bundleDir)
	tarCmd.Stderr = os.Stderr
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
	err = bundleCmd.EnableLive().CaptureStdout(nil).Run()
	if err != nil {
		err = fmt.Errorf("execute bundle: %v", err)
	}
	return bundleCmd.StdoutBytes(), err
}
