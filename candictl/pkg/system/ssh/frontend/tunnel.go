package frontend

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/log"
	"flant/candictl/pkg/system/ssh/cmd"
	"flant/candictl/pkg/system/ssh/session"
	"flant/candictl/pkg/util/retry"
)

type Tunnel struct {
	Session *session.Session
	Type    string // Remote or Local
	Address string
	sshCmd  *exec.Cmd

	stopCh  chan struct{}
	errorCh chan error
}

func NewTunnel(sess *session.Session, ttype string, address string) *Tunnel {
	return &Tunnel{
		Session: sess,
		Type:    ttype,
		Address: address,
		stopCh:  make(chan struct{}, 1),
		errorCh: make(chan error, 1),
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
		t.errorCh <- t.sshCmd.Wait()
	}()

	select {
	case err = <-t.errorCh:
		return fmt.Errorf("cannot open tunnel '%s': %v", t.String(), err)
	case <-tunnelReadyCh:
	}

	return nil
}

func (t *Tunnel) HealthMonitor() error {
	for {
		select {
		case err := <-t.errorCh:
			if err != nil {
				log.ErrorF("Tunnel stopped with an error: %v. ", err)
				log.InfoLn("Restarting a tunnel ...")
			}
			err = retry.StartSilentLoop("tunnel", 5, 5, t.Up)
			if err != nil {
				return err
			}
		case <-t.stopCh:
			_ = t.sshCmd.Process.Kill()
			return nil
		}
	}
}

func (t *Tunnel) Stop() {
	if t == nil {
		return
	}
	if t.Session == nil {
		log.ErrorF("bug: down tunnel '%s': no session", t.String())
		return
	}
	if t.sshCmd != nil {
		t.stopCh <- struct{}{}
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

		if app.IsDebug && text != "" {
			fmt.Printf("%s: %s\n", t.String(), text)
		}
	}
}
