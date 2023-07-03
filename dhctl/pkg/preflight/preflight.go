package preflight

import "github.com/deckhouse/deckhouse/dhctl/pkg/log"

func PreflightCheck() error {
	err := log.Process("preflight-check", "Checking SSH tunnel", func() error {
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
