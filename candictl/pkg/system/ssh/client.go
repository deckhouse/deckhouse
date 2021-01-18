package ssh

import (
	"fmt"

	"github.com/deckhouse/deckhouse/candictl/pkg/system/ssh/frontend"
	"github.com/deckhouse/deckhouse/candictl/pkg/system/ssh/session"
)

type Client struct {
	Settings *session.Session
	Agent    *frontend.Agent
}

func (s *Client) Start() (*Client, error) {
	if s.Settings == nil {
		return nil, fmt.Errorf("Possible bug in ssh client: session should be created before start.")
	}
	s.Agent = frontend.NewAgent(s.Settings)
	return s, s.Agent.Start()
}

// Easy access to frontends

// Tunnel is used to open local (L) and remote (R) tunnels
func (s *Client) Tunnel(ttype string, address string) *frontend.Tunnel {
	return frontend.NewTunnel(s.Settings, ttype, address)
}

// Command is used to run commands on remote server
func (s *Client) Command(name string, arg ...string) *frontend.Command {
	return frontend.NewCommand(s.Settings, name, arg...)
}

// KubeProxy is used to start kubectl proxy and create a tunnel from local port to proxy port
func (s *Client) KubeProxy() *frontend.KubeProxy {
	return frontend.NewKubeProxy(s.Settings)
}

// File is used to upload and download files and directories
func (s *Client) File() *frontend.File {
	return frontend.NewFile(s.Settings)
}

// UploadScript is used to upload script and execute it on remote server
func (s *Client) UploadScript(scriptPath string, args ...string) *frontend.UploadScript {
	return frontend.NewUploadScript(s.Settings, scriptPath, args...)
}

// UploadScript is used to upload script and execute it on remote server
func (s *Client) Check() *frontend.Check {
	return frontend.NewCheck(s.Settings)
}
