package app

import "gopkg.in/alecthomas/kingpin.v2"

var (
	PreflightSkipAll        = false
	PreflightSkipSSHForword = false
)

func DefinePreflight(cmd *kingpin.CmdClause) {
	cmd.Flag("preflight-skip-all-checks", "Skip all preflight checks").
		Envar(configEnvName("PREFLIGHT_SKIP_ALL_CHECKS")).
		BoolVar(&PreflightSkipAll)
	cmd.Flag("preflight-skip-ssh-forward-check", "Skip SSH forward preflight check").
		Envar(configEnvName("PREFLIGHT_SKIP_SSH_FORWARD_CHECK")).
		BoolVar(&PreflightSkipSSHForword)
}
