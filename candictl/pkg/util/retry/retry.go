package retry

import (
	"fmt"
	"time"

	"flant/candictl/pkg/log"
	"flant/candictl/pkg/util/tomb"
)

func StartLoop(name string, attemptsQuantity, waitSeconds int, task func() error) error {
	return log.Process("default", name, func() error {
		for i := 1; i <= attemptsQuantity; i++ {
			select {
			case <-tomb.Ctx().Done():
				return fmt.Errorf("Loop was canceled.\n")
			default:
				if err := task(); err != nil {
					log.Fail(fmt.Sprintf(
						"Attempt #%v of %v |\n\t%s failed, next attempt will be in %vs\n",
						i, attemptsQuantity, name, waitSeconds,
					))
					log.InfoF("\tError: %v\n\n", err)
					<-time.After(time.Duration(waitSeconds) * time.Second)
					continue
				}
				log.Success("Succeeded!\n")
				return nil
			}
		}
		return fmt.Errorf("timeout while %s", name)
	})
}

func StartSilentLoop(name string, attemptsQuantity, waitSeconds int, task func() error) error {
	var err error
	for i := 1; i <= attemptsQuantity; i++ {
		if err = task(); err != nil {
			<-time.After(time.Duration(waitSeconds) * time.Second)
			continue
		}

		return nil
	}
	return fmt.Errorf("timeout while %s: last error: %v", name, err)
}
