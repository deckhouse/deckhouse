package ssh

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"

	"flant/deckhouse-cluster/pkg/app"
)

type Command struct {
	SshClient *SshClient
	Name      string
	Args      []string
	Env       []string
	Echo      bool
	SshArgs   []string

	StdoutSplitter bufio.SplitFunc
	StdoutHandler  func(l string)
	StderrHandler  func(l string)
	OutputHandler  func(l string)
	StdinHandler   func() []byte

	cmd  *exec.Cmd
	stop bool

	WaitCh chan error
}

func (c *Command) Output() ([]byte, []byte, error) {
	if c.SshClient == nil {
		return nil, nil, fmt.Errorf("execute command %s: sshClient is undefined", c.Name)
	}

	c.cmd = c.SshClient.Ssh().
		//	//WithArgs().
		WithCommand(c.Name, c.Args...).Cmd()

	output, err := c.cmd.Output()
	if err != nil {
		//fmt.Printf("%s: %s\n", c.Name, output)
		return output, nil, fmt.Errorf("execute command '%s': %v", c.Name, err)
	}
	return output, nil, nil
}

func (c *Command) CombinedOutput() ([]byte, error) {
	if c.SshClient == nil {
		return nil, fmt.Errorf("execute command %s: sshClient is undefined", c.Name)
	}

	c.cmd = c.SshClient.Ssh().
		//	//WithArgs().
		WithCommand(c.Name, c.Args...).Cmd()

	output, err := c.cmd.CombinedOutput()
	if err != nil {
		//fmt.Printf("%s: %s\n", c.Name, output)
		return output, fmt.Errorf("execute command '%s': %v", c.Name, err)
	}
	return output, nil
}

// Run starts command in background and call *Fn callbacks to handle output
func (c *Command) Start() error {
	var err error
	if c.SshClient == nil {
		return fmt.Errorf("execute command %s: sshClient is undefined", c.Name)
	}

	sshCmd := c.SshClient.Ssh().
		WithArgs(c.SshArgs...).
		WithCommand(c.Name, c.Args...)

	c.cmd = sshCmd.Cmd()

	var stdoutWritePipe *os.File
	var stdoutReadPipe *os.File
	if c.StdoutHandler != nil {
		app.Debugf("setup pipe for stdout hanlder\n")
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
		app.Debugf("setup pipe for stdin hanlder\n")
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

func (c *Command) Stop() error {
	if c.cmd != nil {
		c.stop = true
		c.cmd.Process.Kill()
	}
	return nil
}

func (c *Command) ConsumeLines(r io.Reader, fn func(l string)) {
	//var buf bytes.Buffer
	//tee := io.TeeReader(r, &buf)
	//// echo command stdout to process stdout
	//go func() {
	//	for {
	//		sc := bufio.NewScanner(tee)
	//		sc.Split(bufio.ScanBytes)
	//		for sc.Scan() {
	//			b := sc.Bytes()
	//			os.Stdout.Write(b)
	//		}
	//	}
	//}()

	scanner := bufio.NewScanner(r)
	if c.StdoutSplitter != nil {
		scanner.Split(c.StdoutSplitter)
	}
	for scanner.Scan() {
		text := scanner.Text()

		if fn != nil {
			fn(text)
		}

		if app.IsDebug == 1 && text != "" {
			fmt.Printf("%s: %s\n", c.Name, text)
		}
	}
}
