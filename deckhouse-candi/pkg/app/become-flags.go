package app

import (
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
)

var (
	AskBecomePass = false
	BecomePass    = ""
)

func DefineBecomeFlags(cmd *kingpin.CmdClause) {
	// Ansible compatible
	cmd.Flag("ask-become-pass", "Ask for sudo password before the installation process.").
		Short('K').
		BoolVar(&AskBecomePass)
}

func AskBecomePassword() (err error) {
	if !AskBecomePass {
		return nil
	}
	var data []byte
	if !terminal.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("stdin is not a terminal, error reading password")
	}
	fmt.Print("[sudo] Password: ")
	data, err = terminal.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("read password: %v", err)
	}
	BecomePass = string(data)
	return nil
}
