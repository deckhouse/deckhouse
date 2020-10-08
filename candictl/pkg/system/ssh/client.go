package ssh

import (
	"fmt"

	"flant/candictl/pkg/system/ssh/frontend"
	"flant/candictl/pkg/system/ssh/session"
)

type SSHClient struct {
	Settings *session.Session
	Agent    *frontend.Agent
}

func (s *SSHClient) Start() (*SSHClient, error) {
	if s.Settings == nil {
		return nil, fmt.Errorf("Possible bug in ssh client: session should be created before start.")
	}
	s.Agent = frontend.NewAgent(s.Settings)
	return s, s.Agent.Start()
}

// Easy access to frontends

// Tunnel is used to open local (L) and remote (R) tunnels
func (s *SSHClient) Tunnel(ttype string, address string) *frontend.Tunnel {
	return frontend.NewTunnel(s.Settings, ttype, address)
}

// Command is used to run commands on remote server
func (s *SSHClient) Command(name string, arg ...string) *frontend.Command {
	return frontend.NewCommand(s.Settings, name, arg...)
}

// KubeProxy is used to start kubectl proxy and create a tunnel from local port to proxy port
func (s *SSHClient) KubeProxy() *frontend.KubeProxy {
	return frontend.NewKubeProxy(s.Settings)
}

// File is used to upload and download files and directories
func (s *SSHClient) File() *frontend.File {
	return frontend.NewFile(s.Settings)
}

// UploadScript is used to upload script and execute it on remote server
func (s *SSHClient) UploadScript(scriptPath string, args ...string) *frontend.UploadScript {
	return frontend.NewUploadScript(s.Settings, scriptPath, args...)
}

// UploadScript is used to upload script and execute it on remote server
func (s *SSHClient) Check() *frontend.Check {
	return frontend.NewCheck(s.Settings)
}
