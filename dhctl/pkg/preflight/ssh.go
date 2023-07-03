package preflight

import (
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/frontend"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
)

const (
	DefaultLocalPort  = 20000
	DefaultRemotePort = 20000
)

func CheckSSHTunel(sess *session.Session, localPort, remotePort int) error {
	log.DebugF("Checking ssh tunnel with remote port %s and local port %d\n", remotePort, localPort)
	if localPort == 0 {
		localPort = DefaultLocalPort
	}
	if remotePort == 0 {
		remotePort = DefaultRemotePort
	}

	builder := strings.Builder{}
	builder.WriteString(strconv.Itoa(localPort))
	builder.WriteString(":localhost:")
	builder.WriteString(strconv.Itoa(remotePort))

	tun := frontend.NewTunnel(sess, "L", builder.String())
	err := tun.Up()
	if err != nil {
		return err
	}

	log.DebugLn("Checking ssh tunnel success")
	tun.Stop()
	return nil
}
