package config

import (
	"fmt"
	"os"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func GetConvergeLockLeaseConfig(identity string) client.LeaseLockConfig {
	return client.LeaseLockConfig{
		Name:                         "d8-converge-lock",
		Identity:                     identity,
		Namespace:                    "d8-system",
		LeaseDurationSeconds:         120,
		RenewDurationSeconds:         100,
		TolerableExpiredLeaseSeconds: 60,
		RetryDuration:                2 * time.Second,
		OnRenewError: func(_ error) {
			p, err := os.FindProcess(os.Getpid())
			if err != nil {
				log.ErrorF("Cannot find pid: %v", err)
				return
			}

			err = p.Signal(os.Interrupt)
			if err != nil {
				log.ErrorF("Cannot send interrupt signal: %v", err)
				return
			}
		},
	}
}

func GetLocalConvergeLockIdentity(pref string) string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	return fmt.Sprintf("%v-%v", pref, host)
}
