package input

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ssh/terminal"

	"flant/candictl/pkg/log"
)

func AskForConfirmation(s string, defaultAnswer bool) bool {
	if !terminal.IsTerminal(int(os.Stdin.Fd())) {
		return defaultAnswer
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		log.Warning(fmt.Sprintf("%s? [y/n]: ", s))
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
