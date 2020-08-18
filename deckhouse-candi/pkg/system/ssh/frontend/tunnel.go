package frontend

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/system/ssh/cmd"
	"flant/deckhouse-candi/pkg/system/ssh/session"
)

type Tunnel struct {
	Session *session.Session
	Type    string // Remote or Local
	Address string
	sshCmd  *exec.Cmd
	stop    bool
}

func NewTunnel(sess *session.Session, ttype string, address string) *Tunnel {
	return &Tunnel{
		Session: sess,
		Type:    ttype,
		Address: address,
	}
}

func (t *Tunnel) Up() error {
	if t.Session == nil {
		return fmt.Errorf("up tunnel '%s': sshClient is undefined", t.String())
	}

	t.sshCmd = cmd.NewSsh(t.Session).
		WithArgs(
			//"-f", // start in background - good for scripts, but here we need to do cmd.Process.Kill()
			"-o",
			"ExitOnForwardFailure=yes", // wait for connection establish before
			//"-N",                       // no command
			//"-n", // no stdin
			fmt.Sprintf("-%s", t.Type),
			t.Address,
		).
		WithCommand("echo", "SUCCESS", "&&", "cat").
		Cmd()

	stdoutReadPipe, stdoutWritePipe, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("unable to create os pipe for stdout: %s", err)
	}
	t.sshCmd.Stdout = stdoutWritePipe

	// Create separate stdin pipe to prevent reading from main process Stdin
	stdinReadPipe, _, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("unable to create os pipe for stdin: %s", err)
	}
	t.sshCmd.Stdin = stdinReadPipe

	err = t.sshCmd.Start()
	if err != nil {
		return fmt.Errorf("tunnel up: %v", err)
	}

	tunnelReadyCh := make(chan struct{}, 1)
	tunnelErrorCh := make(chan error, 1)

	go func() {
		//defer wg.Done()
		t.ConsumeLines(stdoutReadPipe, func(l string) {
			if l == "SUCCESS" {
				tunnelReadyCh <- struct{}{}
			}
		})
		app.Debugf("stop line consumer for '%s'", t.String())
	}()

	go func() {
		//defer wg.Done()
		err = t.sshCmd.Wait()
		if t.stop {
			return
		}
		if err != nil {
			tunnelErrorCh <- err
		} else {
			app.Debugf("tunnel '%s' process exited.\n", t.String())
		}
	}()

	select {
	case err = <-tunnelErrorCh:
		return fmt.Errorf("cannot open tunnel '%s': %v", t.String(), err)
	case <-tunnelReadyCh:
	}

	// TODO add tunnel health monitor, restart tunnel if it drops.
	// write to stdinWriter, wait the same text on stdoutReader
	return nil
}

func (t *Tunnel) Stop() {
	if t == nil {
		return
	}
	if t.Session == nil {
		logboek.LogErrorF("bug: down tunnel '%s': no session", t.String())
		return
	}
	if t.sshCmd != nil {
		t.stop = true
		t.sshCmd.Process.Kill()
	}
}

func (t *Tunnel) String() string {
	return fmt.Sprintf("%s:%s", t.Type, t.Address)
}

func (t *Tunnel) ConsumeLines(r io.Reader, fn func(l string)) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()

		if fn != nil {
			fn(text)
		}

		if app.IsDebug == 1 && text != "" {
			fmt.Printf("%s: %s\n", t.String(), text)
		}
	}
}
