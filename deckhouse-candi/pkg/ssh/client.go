package ssh

import (
	"fmt"

	"flant/deckhouse-candi/pkg/ssh/frontend"
	"flant/deckhouse-candi/pkg/ssh/session"
)

type SshClient struct {
	Session *session.Session
	Agent   *frontend.Agent
}

func (s *SshClient) StartSession() (*SshClient, error) {
	if s.Session == nil {
		return nil, fmt.Errorf("bug: session is not created")
	}
	s.Agent = frontend.NewAgent(s.Session)
	return s, s.Agent.Start()
}

func (s *SshClient) StopSession() {
	if s.Agent != nil {
		s.Agent.Stop()
		s.Agent = nil
	}
}

// Easy access to frontends

// Tunnel is used to open local (L) and remote (R) tunnels
func (s *SshClient) Tunnel(ttype string, address string) *frontend.Tunnel {
	return frontend.NewTunnel(s.Session, ttype, address)
}

// Command is used to run commands on remote server
func (s *SshClient) Command(name string, arg ...string) *frontend.Command {
	return frontend.NewCommand(s.Session, name, arg...)
}

// KubeProxy is used to start kubectl proxy and create a tunnel from local port to proxy port
func (s *SshClient) KubeProxy() *frontend.KubeProxy {
	return frontend.NewKubeProxy(s.Session)
}

// File is used to upload and download files and directories
func (s *SshClient) File() *frontend.File {
	return frontend.NewFile(s.Session)
}

// UploadScript is used to upload script and execute it on remote server
func (s *SshClient) UploadScript(scriptPath string, args ...string) *frontend.UploadScript {
	return frontend.NewUploadScript(s.Session, scriptPath, args...)
}

// UploadScript is used to upload script and execute it on remote server
func (s *SshClient) Check() *frontend.Check {
	return frontend.NewCheck(s.Session)
}
