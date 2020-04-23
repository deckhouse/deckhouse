package frontend

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/ssh/cmd"
	"flant/deckhouse-candi/pkg/ssh/session"
	"flant/deckhouse-candi/pkg/ssh/util"
)

type Command struct {
	cmd.Executor

	Session *session.Session

	Name string
	Args []string
	Env  []string

	SshArgs []string

	StdoutSplitter bufio.SplitFunc
	StdoutHandler  func(l string)
	StderrHandler  func(l string)
	OutputHandler  func(l string)
	StdinHandler   func() []byte

	onCommandStart func()

	cmd  *exec.Cmd
	stop bool

	becomeState string

	prepareErr error

	WaitCh chan error
}

func NewCommand(sess *session.Session, name string, arg ...string) *Command {
	return &Command{
		Session: sess,
		Name:    name,
		Args:    arg,
	}
}

func (c *Command) OnCommandStart(fn func()) *Command {
	c.onCommandStart = fn
	return c
}

func (c *Command) Sudo() *Command {
	cmdLine := c.Name + " " + strings.Join(c.Args, " ")
	sudoCmdLine := fmt.Sprintf(`sudo -p SudoPassword -H -S -i bash -c 'echo SUDO-SUCCESS && %s'`, cmdLine)

	args := append(c.SshArgs, []string{
		"-t", // allocate tty to auto kill remote process when ssh process is killed
		"-t", // need to force tty allocation because of stdin is pipe!
	}...)

	c.cmd = cmd.NewSsh(c.Session).
		WithArgs(args...).
		WithCommand(sudoCmdLine).Cmd()

	c.Executor = cmd.Executor{
		Cmd: c.cmd,
	}
	c.WithMatchers(
		util.NewByteSequenceMatcher("SudoPassword"),
		util.NewByteSequenceMatcher("SUDO-SUCCESS").WaitNonMatched(),
	)
	c.OpenStdinPipe()

	passSent := false
	c.WithMatchHandler(func(pattern string) string {
		if pattern == "SudoPassword" {
			if !passSent {
				// send pass through stdin
				app.Debugf("Send become pass to cmd\n")
				c.Executor.Stdin.Write([]byte(app.BecomePass + "\n"))
				passSent = true
			} else {
				// Second prompt is error!
				logboek.LogErrorLn("Bad sudo password, exiting. TODO handle this correctly.")
				os.Exit(1)
			}
			return "reset"
		}
		if pattern == "SUDO-SUCCESS" {
			app.Debugf("Got SUCCESS\n")
			if c.onCommandStart != nil {
				c.onCommandStart()
			}
			return "done"
		}
		return ""
	})
	return c
}

func (c *Command) Cmd() *Command {
	c.cmd = cmd.NewSsh(c.Session).
		WithArgs(c.SshArgs...).
		WithCommand(c.Name, c.Args...).Cmd()

	c.Executor = cmd.Executor{
		Cmd: c.cmd,
	}
	return c
}

func (c *Command) Output() ([]byte, []byte, error) {
	if c.Session == nil {
		return nil, nil, fmt.Errorf("execute command %s: sshClient is undefined", c.Name)
	}

	c.cmd = cmd.NewSsh(c.Session).
		WithArgs(c.SshArgs...).
		WithCommand(c.Name, c.Args...).Cmd()

	output, err := c.cmd.Output()
	if err != nil {
		//fmt.Printf("%s: %s\n", c.Name, output)
		return output, nil, fmt.Errorf("execute command '%s': %v", c.Name, err)
	}
	return output, nil, nil
}

func (c *Command) CombinedOutput() ([]byte, error) {
	if c.Session == nil {
		return nil, fmt.Errorf("execute command %s: sshClient is undefined", c.Name)
	}

	c.cmd = cmd.NewSsh(c.Session).
		//	//WithArgs().
		WithCommand(c.Name, c.Args...).Cmd()

	output, err := c.cmd.CombinedOutput()
	if err != nil {
		//fmt.Printf("%s: %s\n", c.Name, output)
		return output, fmt.Errorf("execute command '%s': %v", c.Name, err)
	}
	return output, nil
}

// Run starts command and waits until it exits
func (c *Command) RunOld() error {
	if c.Session == nil {
		return fmt.Errorf("execute command %s: sshClient is undefined", c.Name)
	}

	sshCmd := cmd.NewSsh(c.Session).
		WithArgs(c.SshArgs...).
		WithCommand(c.Name, c.Args...)

	c.cmd = sshCmd.Cmd()
	c.cmd.Stdout = os.Stdout
	return c.cmd.Run()
}

// LiveOutput wait until command exits and write stdin and stderr on console and returns a capture a stdout.
func (c *Command) LiveOutput() (stdout []byte, err error) {
	if c.Session == nil {
		return nil, fmt.Errorf("execute command %s: sshClient is undefined", c.Name)
	}

	sshCmd := cmd.NewSsh(c.Session).
		WithArgs(c.SshArgs...).
		WithCommand(c.Name, c.Args...)

	c.cmd = sshCmd.Cmd()
	c.cmd.Stderr = os.Stderr

	//c.cmd.Stderr = os.Stderr
	var stdoutWritePipe *os.File
	var stdoutReadPipe *os.File
	stdoutReadPipe, stdoutWritePipe, err = os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("unable to create os pipe for stdout: %s", err)
	}
	c.cmd.Stdout = stdoutWritePipe

	var stdoutOutput bytes.Buffer

	go func() {
		buf := make([]byte, 16)
		for {
			n, err := stdoutReadPipe.Read(buf)
			os.Stdout.Write(buf[:n])
			stdoutOutput.Write(buf[:n])
			if err == io.EOF {
				break
			}
		}
	}()

	err = c.cmd.Run()
	return stdoutOutput.Bytes(), err
}

// Start runs command in background and call *Fn callbacks to handle output
func (c *Command) StartOld() error {
	var err error
	if c.Session == nil {
		return fmt.Errorf("execute command %s: sshClient is undefined", c.Name)
	}

	sshCmd := cmd.NewSsh(c.Session).
		WithArgs(c.SshArgs...).
		WithCommand(c.Name, c.Args...)

	c.cmd = sshCmd.Cmd()

	var stdoutWritePipe *os.File
	var stdoutReadPipe *os.File
	if c.StdoutHandler != nil {
		app.Debugf("setup pipe for stdout handler\n")
		stdoutReadPipe, stdoutWritePipe, err = os.Pipe()
		if err != nil {
			return fmt.Errorf("unable to create os pipe for stdout: %s", err)
		}
		c.cmd.Stdout = stdoutWritePipe
	} else {
		c.cmd.Stdout = os.Stdout
	}

	var stdinWritePipe *os.File
	var stdinReadPipe *os.File
	if c.StdinHandler != nil {
		app.Debugf("setup pipe for stdin handler\n")
		stdinReadPipe, stdinWritePipe, err = os.Pipe()
		if err != nil {
			return fmt.Errorf("unable to create os pipe for stdin: %s", err)
		}
		c.cmd.Stdin = stdinReadPipe
	} else {
		c.cmd.Stdin = os.Stdin
	}

	c.cmd.Stderr = os.Stderr

	err = c.cmd.Start()
	if err != nil {
		return fmt.Errorf("start subprocess '%s': %v", c.Name, err)
	}

	c.WaitCh = make(chan error, 1)
	go func() {
		err := c.cmd.Wait()
		if c.stop {
			return
		}
		c.WaitCh <- err
		close(c.WaitCh)
	}()

	if c.StdoutHandler != nil {
		go func() {
			app.Debugf("start line consumer\n")
			//defer wg.Done()
			c.ConsumeLines(stdoutReadPipe, c.StdoutHandler)
			app.Debugf("stop line consumer for '%s'\n", c.Name)
		}()
	}

	if c.StdinHandler != nil {
		go func() {
			app.Debugf("start stdin handler loop\n")
			for {
				buf := c.StdinHandler()
				if len(buf) == 0 {
					break
				}
				stdinWritePipe.Write(buf)
			}
		}()
	}

	return nil
}

//func (c *Command) Stop() error {
//	if c.cmd != nil {
//		c.stop = true
//		c.cmd.Process.Kill()
//	}
//	return nil
//}
