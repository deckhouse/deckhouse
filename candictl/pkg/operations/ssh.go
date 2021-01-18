package operations

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/deckhouse/deckhouse/candictl/pkg/app"
	"github.com/deckhouse/deckhouse/candictl/pkg/log"
)

func AskBecomePassword() (err error) {
	if !app.AskBecomePass {
		return nil
	}

	fd := int(os.Stdin.Fd())

	var data []byte
	if !terminal.IsTerminal(fd) {
		return fmt.Errorf("stdin is not a terminal, error reading password")
	}

	log.InfoF("[sudo] Password: ")

	data, err = terminal.ReadPassword(fd)
	log.InfoLn()

	if err != nil {
		return fmt.Errorf("read password: %v", err)
	}

	app.BecomePass = string(data)
	return nil
}
