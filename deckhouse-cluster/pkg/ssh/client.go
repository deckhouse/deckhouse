package ssh

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"flant/deckhouse-cluster/pkg/app"
)

var SshAgentPath = "ssh-agent"
var SshAddPath = "ssh-add"
var SshPath = "ssh"

func ParseSshPrivateKeyPaths(paths string) ([]string, error) {
	res := make([]string, 0)
	if paths == "" {
		return res, nil
	}
	keys := strings.Split(paths, ",")
	for _, k := range keys {
		if strings.HasPrefix(k, "~") {
			home := os.Getenv("HOME")
			if home == "" {
				return nil, fmt.Errorf("HOME is not defined for key '%s'", k)
			}
			k = strings.Replace(k, "~", home, 1)
		}

		keyPath, err := filepath.Abs(k)
		if err != nil {
			return nil, fmt.Errorf("get absolute path for '%s': %v", k, err)
		}
		res = append(res, keyPath)
	}
	return res, nil
}

type SshClient struct {
	PrivateKeys []string
	Host        string
	User        string
	BastionHost string
	BastionUser string
	ExtraArgs   string

	//SshAgentPid      string
	SshAgentAuthSock string

	sshAgent *exec.Cmd
}

func (s *SshClient) StartSshAgent() error {
	if len(s.PrivateKeys) == 0 {
		s.SshAgentAuthSock = os.Getenv("SSH_AUTH_SOCK")
		return nil
	}

	/*
	  # launch temporary ssh agent
	  eval "$(ssh-agent)" > /dev/null
	  trap 'kill '"$SSH_AGENT_PID" EXIT
	  # shellcheck disable=SC2154
	  for f in ${ssh_private_keys_paths//,/ } ; do
	    # shellcheck disable=SC2086
	    ssh-add ${f/#\~/$HOME} || exit $?
	  done
	*/

	env := []string{
		//fmt.Sprintf("TILLER_NAMESPACE=%s", options.Namespace),
		//fmt.Sprintf("TILLER_HISTORY_MAX=%d", options.HistoryMax),
	}

	args := []string{
		"-D",
		//"-listen",
		//fmt.Sprintf("%s:%d", options.ListenAddress, options.ListenPort),
		//"-probe-listen",
		//fmt.Sprintf("%s:%d", options.ProbeListenAddress, options.ProbeListenPort),
	}

	stdoutReadPipe, stdoutWritePipe, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("unable to create os pipe for stdout: %s", err)
	}

	s.sshAgent = exec.Command(SshAgentPath, args...)
	s.sshAgent.Env = append(os.Environ(), env...)
	s.sshAgent.Dir = "/"
	s.sshAgent.Stdout = stdoutWritePipe

	err = s.sshAgent.Start()
	if err != nil {
		return fmt.Errorf("start ssh-agent subprocess: %v", err)
	}

	go func() {
		//defer wg.Done()
		s.ConsumeSshAgentLines(stdoutReadPipe)
	}()

	// TODO create a channel to distinguish between agent error and planned stop
	go func() {
		//defer wg.Done()
		err = s.sshAgent.Wait()
		if err != nil {
			fmt.Sprintf("Ssh-agent process exited, now stop. Wait error: %v", err)
		} else {
			fmt.Sprintf("Ssh-agent process exited, now stop.")
		}
		os.Exit(1)
	}()

	// wait for ssh agent pid
	success := false
	maxWait := 1000
	retries := 0
	t := time.NewTicker(5 * time.Millisecond)
	for {
		<-t.C
		//app.Debugf("retry %d\n", retries)
		if s.SshAgentAuthSock != "" {
			app.Debugf("ssh-agent: SSH_AUTH_SOCK=%s\n", s.SshAgentAuthSock)
			success = true
			break
		}
		retries++
		if retries > maxWait {
			break
		}
	}
	t.Stop()

	if !success {
		return fmt.Errorf("cannot get pid and auth sock path for ssh-agent")
	}

	return nil
}

func (s *SshClient) ConsumeSshAgentLines(r io.Reader) {
	authSockRe := regexp.MustCompile(`SSH_AUTH_SOCK=(.*?);`)
	//pidRe := regexp.MustCompile(`Agent\ pid\ (\d+);`)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()
		// try to find auth sock
		m := authSockRe.FindStringSubmatch(text)
		if len(m) == 2 && m[1] != "" {
			s.SshAgentAuthSock = m[1]
		}

		if app.IsDebug == 1 && text != "" {
			fmt.Printf("ssh-agent: %s\n", text)
		}
	}
}

func (s *SshClient) StopSshAgent() error {
	if s.sshAgent != nil {
		s.sshAgent.Process.Kill()
	}
	return nil
}

// TODO replace with x/crypto/ssh/agent ?
func (s *SshClient) AddKeys() error {
	for _, k := range s.PrivateKeys {
		app.Debugf("add key %s\n", k)
		args := []string{
			k,
		}
		env := []string{
			fmt.Sprintf("SSH_AUTH_SOCK=%s", s.SshAgentAuthSock),
		}
		cmd := exec.Command(SshAddPath, args...)
		cmd.Env = append(os.Environ(), env...)

		output, err := cmd.Output()

		if err != nil {
			return fmt.Errorf("ssh-add: %v", err)
		}
		str := string(output)
		if str != "" && str != "\n" {
			fmt.Printf("ssh-add: %s\n", output)
		}
	}

	if app.IsDebug == 1 {
		app.Debugf("list added keys\n")
		env := []string{
			fmt.Sprintf("SSH_AUTH_SOCK=%s", s.SshAgentAuthSock),
		}
		cmd := exec.Command(SshAddPath, "-l")
		cmd.Env = append(os.Environ(), env...)

		output, err := cmd.CombinedOutput()

		if err != nil {
			return fmt.Errorf("ssh-add -l: %v", err)
		}
		str := string(output)
		if str != "" && str != "\n" {
			fmt.Printf("ssh-add -l: %s\n", output)
		}
	}

	return nil
}

// TODO move to Ssh type
func (s *SshClient) ExecuteCmd(cmd *exec.Cmd) error {
	env := []string{
		fmt.Sprintf("SSH_AUTH_SOCK=%s", s.SshAgentAuthSock),
	}

	// ssh connection settings
	//   ANSIBLE_SSH_ARGS="${ANSIBLE_SSH_ARGS:-"-C
	//   -o ControlMaster=auto
	//  -o ControlPersist=600s"}
	//
	// -o StrictHostKeyChecking=accept-new
	// -o UserKnownHostsFile=$(pwd)/.konverge/$terraform_workspace/.ssh_known_hosts"
	args := []string{
		// ssh args for bastion here
		"-C", // compression
		"-o",
		"ControlMaster=auto",
		"-o",
		"ControlPersist=600s",
		"-o",
		"StrictHostKeyChecking=accept-new",
		"-o",
		"UserKnownHostsFile=.ssh_known_hosts",
	}

	if s.ExtraArgs != "" {
		extraArgs := strings.Split(s.ExtraArgs, " ")
		if len(extraArgs) > 0 {
			args = append(args, extraArgs...)
		}
	}

	// add bastion options
	//  if [[ "x$ssh_bastion_host" != "x" ]] ; then
	//    export ANSIBLE_SSH_ARGS="${ANSIBLE_SSH_ARGS}
	//   -o ProxyCommand='ssh ${ssh_bastion_user:-$USER}@$ssh_bastion_host -W %h:%p'"
	//  fi
	if s.BastionHost != "" {
		bastion := s.BastionHost
		if s.BastionUser != "" {
			bastion = s.BastionUser + "@" + s.BastionHost
		}
		args = append(args, []string{
			"-o",
			fmt.Sprintf("ProxyCommand=ssh %s -W %%h:%%p", bastion), // note that single quotes is not needed here
		}...)
	}

	// add destination
	if s.User != "" {
		args = append(args, []string{
			"-l",
			s.User,
		}...)
	}

	args = append(args, s.Host)

	// TODO move command arguments to Command
	// Add command
	args = append(args, "--")
	//args = append(args, cmd.Path)
	args = append(args, cmd.Args...)

	sshCmd := exec.Command("ssh", args...)

	// TODO oops: sign_and_send_pubkey: signing failed: agent refused operation

	//sshCmd.Env = append(os.Environ(), cmd.Env...)
	//sshCmd.Env = append(sshCmd.Env, env...)

	// set auth sock
	//envs := make([]string, 0)
	//for _, e := range sshCmd.Env {

	//}

	sshCmd.Env = env

	app.Debugf("exec ssh: %s\n %#v\n", sshCmd.String(), sshCmd)

	// TODO move to Command. Tunnel do not need an Output!
	output, err := sshCmd.CombinedOutput()

	if err != nil {
		fmt.Printf("ssh: %s\n", output)
		return fmt.Errorf("ssh: %v", err)
	}
	fmt.Printf("ssh: %s", output)

	return nil
}

// Ssh is a ssh command with connections settings.
func (s *SshClient) Ssh() *Ssh {
	return &Ssh{
		SshClient: s,
	}
}

// Tunnel is an object to open tunnels
func (s *SshClient) Tunnel(ttype string, address string) *Tunnel {
	return &Tunnel{
		SshClient: s,
		Type:      ttype,
		Address:   address,
	}
}

// Command is used to run commands on remote server
func (s *SshClient) Command(name string, arg ...string) *Command {
	return &Command{
		SshClient: s,
		Name:      name,
		Args:      arg,
	}
}

// Command is used to run commands on remote server
func (s *SshClient) KubeProxy() *KubeProxy {
	return &KubeProxy{
		SshClient: s,
	}
}
