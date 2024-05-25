package discovery

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/state"
)

type StaticDiscoverer struct {
	sshClient *ssh.Client
}

func NewStaticDiscoverer(st state.SSHState) (*StaticDiscoverer, error) {
	f, err := os.CreateTemp("", "static-discoverer")
	if err != nil {
		return nil, err
	}
	_, err = io.WriteString(f, st.Private)
	if err != nil {
		return nil, err
	}

	privKeyFileName := f.Name()

	err = f.Close()
	if err != nil {
		return nil, err
	}

	s := &session.Session{
		User: st.User,
		Port: "22",

		BastionHost: st.BastionHost,
		BastionUser: st.BastionUser,
		BastionPort: "22",
	}

	s.SetAvailableHosts([]string{st.Host})

	cl := ssh.NewClient(s, []session.AgentPrivateKey{{Key: privKeyFileName}})
	return &StaticDiscoverer{
		sshClient: cl,
	}, nil
}

func (d *StaticDiscoverer) GetInternalNetworkCIDR() ([]string, error) {
	type addrInfo struct {
		Family    string `json:"family"`
		Local     string `json:"local"`
		PrefixLen int    `json:"prefixlen"`
	}
	type inet struct {
		LinkType string     `json:"link_type"`
		AddrInfo []addrInfo `json:"addr_info"`
	}

	c, err := d.sshClient.Start()
	if err != nil {
		return nil, err
	}

	out, _, err := c.Command("ip", "--json", "a").Output()
	if err != nil {
		return nil, err
	}

	var links []inet
	err = json.Unmarshal(out, &links)
	if err != nil {
		return nil, err
	}

	cidrs := make([]string, 0)

	for _, l := range links {
		if l.LinkType == "loopback" {
			continue
		}

		for _, i := range l.AddrInfo {
			if i.Family == "inet6" {
				continue
			}
			_, ipNet, err := net.ParseCIDR(fmt.Sprintf("%s/%d", i.Local, i.PrefixLen))
			if err != nil {
				return nil, err
			}

			cidrs = append(cidrs, ipNet.String())
		}
	}

	return cidrs, nil
}
