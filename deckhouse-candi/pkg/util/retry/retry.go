package retry

import (
	"fmt"
	"time"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/log"
)

func StartLoop(name string, attemptsQuantity, waitSeconds int, task func() error) error {
	return logboek.LogProcess(name, log.BoldOptions(), func() error {
		for i := 1; i <= attemptsQuantity; i++ {
			if err := task(); err != nil {
				logboek.LogInfoF("❌ Attempt #%v of %v |\n\t%s failed, next attempt will be in %vs\n", i, attemptsQuantity, name, waitSeconds)
				logboek.LogInfoF("\nError: %v\n\n", err)
				<-time.After(time.Duration(waitSeconds) * time.Second)
				continue
			}

			logboek.LogInfoLn("✅ Succeeded!")
			return nil
		}
		return fmt.Errorf("timeout while %s", name)
	})
}

func StartSilentLoop(name string, attemptsQuantity, waitSeconds int, task func() error) error {
	for i := 1; i <= attemptsQuantity; i++ {
		if err := task(); err != nil {
			<-time.After(time.Duration(waitSeconds) * time.Second)
			continue
		}

		return nil
	}
	return fmt.Errorf("timeout while %s", name)
}
