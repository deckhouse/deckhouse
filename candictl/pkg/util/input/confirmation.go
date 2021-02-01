package input

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/deckhouse/deckhouse/candictl/pkg/log"
)

type Confirmation struct {
	message       string
	defaultAnswer bool
}

func NewConfirmation() *Confirmation {
	return &Confirmation{message: "Should we proceed?"}
}

func (c *Confirmation) WithYesByDefault() *Confirmation {
	c.defaultAnswer = true
	return c
}

func (c *Confirmation) WithMessage(m string) *Confirmation {
	c.message = m
	return c
}

func (c *Confirmation) Ask() bool {
	if !terminal.IsTerminal(int(os.Stdin.Fd())) {
		return c.defaultAnswer
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		log.WarnF(fmt.Sprintf("%s [y/n]: ", c.message))
		line, _, err := reader.ReadLine()
		if err != nil {
			log.ErrorF("can't read from stdin: %v\n", err)
			return false
		}

		response := strings.ToLower(strings.TrimSpace(string(line)))

		if response == "y" || response == "yes" {
			log.InfoF("\r")
			return true
		} else if response == "n" || response == "no" {
			log.InfoF("\r")
			return false
		}
		log.InfoF("\r")
	}
}
