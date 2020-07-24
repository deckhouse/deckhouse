package signal

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/flant/logboek"
)

// WaitForProcessInterruption wait for SIGINT or SIGTERM and run a callback function.
//
// First signal start a callback function, which should call os.Exit(0).
// Next signal will call os.Exit(128 + signal-value).
// If no cb is given,
func WaitForProcessInterruption(cb ...func()) {
	allowedCount := 1
	interruptCh := make(chan os.Signal, 1)

	forcedExit := func(s os.Signal) {
		logboek.LogErrorF("Forced shutdown by '%s' signal\n", s.String())

		Exit(s)
	}

	signal.Notify(interruptCh, syscall.SIGINT, syscall.SIGTERM)
	for {
		sig := <-interruptCh
		allowedCount--
		switch allowedCount {
		case 0:
			if len(cb) > 0 {
				logboek.LogWarnF("Grace shutdown by '%s' signal\n", sig.String())
				cb[0]()
				Exit(sig)
			} else {
				forcedExit(sig)
			}
		case -1:
			forcedExit(sig)
		}
	}
}

func Exit(s os.Signal) {
	signum := 0
	v, ok := s.(syscall.Signal)
	if ok {
		signum = int(v)
	}
	os.Exit(128 + signum)
}
